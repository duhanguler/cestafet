package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Roles and statuses of a session.
const (
	RoleMember      = "member"
	RoleCoordinator = "coordinator"

	StatusLive = "live"
	StatusIdle = "idle" // derived: live but not seen within IdleAfter
	StatusGone = "gone"
)

// IdleAfter is how long a live session may stay silent before it is shown as idle.
const IdleAfter = 15 * time.Minute

// Session is one registered agent session.
type Session struct {
	ID        string    `json:"id"`
	ClaudePID int       `json:"claude_pid"`
	Profile   string    `json:"profile"`
	Project   string    `json:"project"`
	Repo      string    `json:"repo"`
	Cwd       string    `json:"cwd"`
	Branch    string    `json:"branch"`
	Summary   string    `json:"summary"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	LastSeen  time.Time `json:"last_seen"`
}

const sessionCols = `id, claude_pid, profile, project, repo, cwd, branch, summary, role, status, started_at, last_seen`

func scanSession(row interface{ Scan(...any) error }) (Session, error) {
	var s Session
	var started, seen int64
	err := row.Scan(&s.ID, &s.ClaudePID, &s.Profile, &s.Project, &s.Repo, &s.Cwd,
		&s.Branch, &s.Summary, &s.Role, &s.Status, &started, &seen)
	if err != nil {
		return Session{}, err
	}
	s.StartedAt = time.Unix(started, 0)
	s.LastSeen = time.Unix(seen, 0)
	return s, nil
}

// effectiveStatus turns a stored "live" into "idle" when the session went quiet.
func (s *Store) effectiveStatus(sess Session) Session {
	if sess.Status == StatusLive && s.now().Sub(sess.LastSeen) > IdleAfter {
		sess.Status = StatusIdle
	}
	return sess
}

// Register inserts a session or refreshes it. Summary and role survive a
// re-registration (resume, /clear) of the same session id. It reports whether
// the session is new.
func (s *Store) Register(in Session) (bool, error) {
	now := s.unix()
	res, err := s.db.Exec(`
		INSERT INTO sessions (`+sessionCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING`,
		in.ID, in.ClaudePID, in.Profile, in.Project, in.Repo, in.Cwd, in.Branch,
		RoleMember, StatusLive, now, now)
	if err != nil {
		return false, fmt.Errorf("insert session: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		return true, nil
	}
	_, err = s.db.Exec(`
		UPDATE sessions SET claude_pid = ?, profile = ?, project = ?, repo = ?, cwd = ?,
			branch = ?, status = ?, last_seen = ?
		WHERE id = ?`,
		in.ClaudePID, in.Profile, in.Project, in.Repo, in.Cwd, in.Branch, StatusLive, now, in.ID)
	if err != nil {
		return false, fmt.Errorf("refresh session: %w", err)
	}
	return false, nil
}

// Touch updates last_seen and revives the session.
func (s *Store) Touch(id string) error {
	_, err := s.db.Exec(`UPDATE sessions SET last_seen = ?, status = ? WHERE id = ?`, s.unix(), StatusLive, id)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// End marks a session as gone and drops its coordinator role.
func (s *Store) End(id string) error {
	_, err := s.db.Exec(`UPDATE sessions SET status = ?, role = ?, last_seen = ? WHERE id = ?`,
		StatusGone, RoleMember, s.unix(), id)
	if err != nil {
		return fmt.Errorf("end session: %w", err)
	}
	return nil
}

// Get returns one session.
func (s *Store) Get(id string) (Session, error) {
	row := s.db.QueryRow(`SELECT `+sessionCols+` FROM sessions WHERE id = ?`, id)
	sess, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	return s.effectiveStatus(sess), nil
}

// ByClaudePID returns the most recently seen non-gone session owned by the
// given claude process. It links an MCP server process to its session.
func (s *Store) ByClaudePID(pid int) (Session, error) {
	row := s.db.QueryRow(`SELECT `+sessionCols+` FROM sessions
		WHERE claude_pid = ? AND status != ? ORDER BY last_seen DESC LIMIT 1`, pid, StatusGone)
	sess, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("find session by pid: %w", err)
	}
	return s.effectiveStatus(sess), nil
}

// List returns the non-gone sessions of a project, coordinator first.
func (s *Store) List(project string) ([]Session, error) {
	rows, err := s.db.Query(`SELECT `+sessionCols+` FROM sessions
		WHERE project = ? AND status != ?
		ORDER BY role = 'coordinator' DESC, last_seen DESC`, project, StatusGone)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		out = append(out, s.effectiveStatus(sess))
	}
	return out, rows.Err()
}

// Coordinator returns the project's coordinator, or ErrNotFound.
func (s *Store) Coordinator(project string) (Session, error) {
	row := s.db.QueryRow(`SELECT `+sessionCols+` FROM sessions
		WHERE project = ? AND role = ? AND status != ? LIMIT 1`, project, RoleCoordinator, StatusGone)
	sess, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get coordinator: %w", err)
	}
	return s.effectiveStatus(sess), nil
}

// MakeCoordinator gives the session the coordinator role of its project and
// returns the previous coordinator's id ("" if none).
func (s *Store) MakeCoordinator(id string) (string, error) {
	sess, err := s.Get(id)
	if err != nil {
		return "", err
	}
	prev := ""
	if c, err := s.Coordinator(sess.Project); err == nil && c.ID != id {
		prev = c.ID
	}
	tx, err := s.db.Begin()
	if err != nil {
		return "", fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE sessions SET role = ? WHERE project = ? AND role = ?`,
		RoleMember, sess.Project, RoleCoordinator); err != nil {
		return "", fmt.Errorf("clear coordinator: %w", err)
	}
	if _, err := tx.Exec(`UPDATE sessions SET role = ? WHERE id = ?`, RoleCoordinator, id); err != nil {
		return "", fmt.Errorf("set coordinator: %w", err)
	}
	return prev, tx.Commit()
}

// SetSummary records what the session is working on.
func (s *Store) SetSummary(id, summary string) error {
	_, err := s.db.Exec(`UPDATE sessions SET summary = ? WHERE id = ?`, summary, id)
	if err != nil {
		return fmt.Errorf("set summary: %w", err)
	}
	return nil
}

// SetProject moves a session to another project; a coordinator role does not move with it.
func (s *Store) SetProject(id, project string) error {
	_, err := s.db.Exec(`UPDATE sessions SET project = ?,
		role = CASE WHEN project = ? THEN role ELSE ? END WHERE id = ?`, project, project, RoleMember, id)
	if err != nil {
		return fmt.Errorf("set project: %w", err)
	}
	return nil
}

// RecordPrompt stores a user prompt for provenance and returns its id.
// Prompts older than seven days are pruned.
func (s *Store) RecordPrompt(sessionID, text string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO prompts (session_id, text, at) VALUES (?, ?, ?)`, sessionID, text, s.unix())
	if err != nil {
		return 0, fmt.Errorf("record prompt: %w", err)
	}
	_, _ = s.db.Exec(`DELETE FROM prompts WHERE at < ?`, s.unix()-7*24*3600)
	return res.LastInsertId()
}

// Prompt is a user prompt recorded by the UserPromptSubmit hook.
type Prompt struct {
	ID   int64
	Text string
	At   time.Time
}

// LastPrompt returns the session's most recent user prompt.
func (s *Store) LastPrompt(sessionID string) (Prompt, error) {
	var p Prompt
	var at int64
	err := s.db.QueryRow(`SELECT id, text, at FROM prompts WHERE session_id = ? ORDER BY id DESC LIMIT 1`,
		sessionID).Scan(&p.ID, &p.Text, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return Prompt{}, ErrNotFound
	}
	if err != nil {
		return Prompt{}, fmt.Errorf("last prompt: %w", err)
	}
	p.At = time.Unix(at, 0)
	return p, nil
}

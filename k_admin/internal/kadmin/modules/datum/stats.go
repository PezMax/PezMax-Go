package datum

import (
	"github.com/GoAdminGroup/go-admin/modules/db"
)

// UserStats aggregates the profile stat card and the upload ranking. The
// ptmj_user.count column is the authoritative upload counter (maintained by
// upload/delete transactions); the other counts are live queries.
type UserStats struct {
	conn db.Connection
}

func NewUserStats(conn db.Connection) *UserStats {
	return &UserStats{conn: conn}
}

// UploaderRank is one row of the upload leaderboard (ptmj_user.count desc).
type UploaderRank struct {
	UserID   int64  `json:"userId"`
	UserName string `json:"userName"`
	Avatar   string `json:"avatar"`
	Uploads  int64  `json:"uploads"`
}

// TopUploaders ranks active users by upload count (cached upstream in Redis
// by the desktop rank endpoint).
func (s *UserStats) TopUploaders(limit int) ([]UploaderRank, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.conn.Query(`SELECT user_id, user_name, avatar, count FROM ptmj_user
		WHERE status = '1' AND count > 0 ORDER BY count DESC, user_id ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	ranks := make([]UploaderRank, 0, len(rows))
	for _, row := range rows {
		ranks = append(ranks, UploaderRank{
			UserID:   ScanInt64(row["user_id"]),
			UserName: ScanString(row["user_name"]),
			Avatar:   ScanString(row["avatar"]),
			Uploads:  ScanInt64(row["count"]),
		})
	}
	return ranks, nil
}

// ProfileStats returns the four counters of the personal center.
func (s *UserStats) ProfileStats(userID int64) (uploads, downloads, fileFavorites, bookmarkFavorites int64, err error) {
	if err = s.count(&uploads, `SELECT count FROM ptmj_user WHERE user_id = ?`, userID); err != nil {
		return
	}
	if err = s.count(&downloads, `SELECT count(*) AS count FROM ptmj_file_download WHERE user_id = ?`, userID); err != nil {
		return
	}
	if err = s.count(&fileFavorites, `SELECT count(*) AS count FROM ptmj_file_favorite WHERE user_id = ?`, userID); err != nil {
		return
	}
	if err = s.count(&bookmarkFavorites, `SELECT count(*) AS count FROM ptmj_bookmark_favorite WHERE user_id = ?`, userID); err != nil {
		return
	}
	return
}

// IncrementUploads / DecrementUploads maintain the redundant upload counter
// transactionally alongside ptmj_file insert/delete.
func (s *UserStats) IncrementUploads(userID int64) error {
	_, err := s.conn.Exec(`UPDATE ptmj_user SET count = count + 1, update_time = CURRENT_TIMESTAMP WHERE user_id = ?`, userID)
	return err
}

func (s *UserStats) DecrementUploads(userID int64) error {
	_, err := s.conn.Exec(`UPDATE ptmj_user SET count = GREATEST(count - 1, 0), update_time = CURRENT_TIMESTAMP WHERE user_id = ?`, userID)
	return err
}

func (s *UserStats) count(target *int64, query string, args ...interface{}) error {
	rows, err := s.conn.Query(query, args...)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		*target = 0
		return nil
	}
	*target = ScanInt64(rows[0]["count"])
	return nil
}

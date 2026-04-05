package store

import (
	"database/sql"
	"time"

	"github.com/CameronJHall/docketeer/internal/task"
)

type scanner interface {
	Scan(dest ...any) error
}

func scanItem(s scanner) (*task.Item, error) {
	var item task.Item
	var kindStr string
	var prioritySQL sql.NullInt64
	var statusSQL sql.NullString
	var dueDateSQL sql.NullInt64
	var createdUnix, updatedUnix int64

	err := s.Scan(
		&item.ID,
		&kindStr,
		&item.Title,
		&item.Description,
		&prioritySQL,
		&statusSQL,
		&item.Project,
		&dueDateSQL,
		&createdUnix,
		&updatedUnix,
	)
	if err != nil {
		return nil, err
	}

	item.Kind = task.ItemKind(kindStr)
	item.CreatedAt = time.Unix(createdUnix, 0)
	item.UpdatedAt = time.Unix(updatedUnix, 0)

	if prioritySQL.Valid {
		item.Priority = new(task.Priority(prioritySQL.Int64))
	}
	if statusSQL.Valid {
		item.Status = new(task.Status(statusSQL.String))
	}
	if dueDateSQL.Valid {
		item.DueDate = new(time.Unix(dueDateSQL.Int64, 0))
	}

	return &item, nil
}

func priorityToSQL(p *task.Priority) any {
	if p == nil {
		return nil
	}
	return int64(*p)
}

func statusToSQL(s *task.Status) any {
	if s == nil {
		return nil
	}
	return string(*s)
}

func timeToSQL(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Unix()
}

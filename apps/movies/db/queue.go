package db

import "time"

type Queue struct {
	order []int64
	byID  map[int64]*QueuedMovie
}

func (queue *Queue) Get(id int64) *QueuedMovie {
	movie := queue.byID[id]
	return movie
}

type QueuedMovie struct {
	ID       int64 `gorm:"column:privateKey"`
	QueuedAt string
}

func (qm *QueuedMovie) Less(other *QueuedMovie) bool {
	return qm.QueuedAt < other.QueuedAt
}

func (db *DB) RemoveFromQueue(id int64) error {
	if err := db.Create(&QueuedMovie{ID: id, QueuedAt: time.Now().Format(time.DateTime)}).Error; err != nil {
		return err
	}
	return nil
}

func (db *DB) QueueMovie(id int64) error {
	if err := db.Create(&QueuedMovie{ID: id, QueuedAt: time.Now().Format(time.DateTime)}).Error; err != nil {
		return err
	}
	return nil
}

func (db *DB) Queue() (*Queue, error) {
	queuedMovies := []*QueuedMovie{}
	tx := db.Table("queued_movies").Find(&queuedMovies).Order("queued_at asc")
	if err := tx.Error; err != nil {
		return nil, err
	}
	queue := &Queue{
		byID:  make(map[int64]*QueuedMovie, len(queuedMovies)),
		order: make([]int64, len(queuedMovies)),
	}
	for i, movie := range queuedMovies {
		queue.byID[movie.ID] = movie
		queue.order[i] = movie.ID
	}
	return queue, nil
}

package repository_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"social-notif/internal/domain"
	"social-notif/internal/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGormMessageRepository_Create(t *testing.T) {
	db, mock, cleanup := newMockGormDB(t)
	defer cleanup()

	repo := repository.NewMessageRepository(db)
	msg := &domain.Message{
		ID:          uuid.New(),
		PhoneNumber: "+15551234567",
		Body:        "hello",
		Status:      domain.MessageStatusPending,
		RetryCount:  0,
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "messages"`)).
		WithArgs(
			msg.ID,
			msg.PhoneNumber,
			msg.Body,
			msg.Status,
			msg.RetryCount,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.Create(context.Background(), msg); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGormMessageRepository_UpdateStatus(t *testing.T) {
	db, mock, cleanup := newMockGormDB(t)
	defer cleanup()

	repo := repository.NewMessageRepository(db)
	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "messages" SET "status"=$1,"updated_at"=$2 WHERE id = $3`)).
		WithArgs(domain.MessageStatusSent, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpdateStatus(context.Background(), id, domain.MessageStatusSent); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGormMessageRepository_UpdateStatus_NotFound(t *testing.T) {
	db, mock, cleanup := newMockGormDB(t)
	defer cleanup()

	repo := repository.NewMessageRepository(db)
	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "messages" SET "status"=$1,"updated_at"=$2 WHERE id = $3`)).
		WithArgs(domain.MessageStatusSent, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := repo.UpdateStatus(context.Background(), id, domain.MessageStatusSent)
	if !errors.Is(err, repository.ErrMessageNotFound) {
		t.Fatalf("UpdateStatus() error = %v, want ErrMessageNotFound", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGormMessageRepository_GetByID(t *testing.T) {
	db, mock, cleanup := newMockGormDB(t)
	defer cleanup()

	repo := repository.NewMessageRepository(db)
	id := uuid.New()
	createdAt := time.Now().UTC()
	updatedAt := createdAt.Add(time.Minute)

	rows := sqlmock.NewRows([]string{
		"id",
		"phone_number",
		"body",
		"status",
		"provider_response",
		"retry_count",
		"created_at",
		"updated_at",
	}).AddRow(
		id,
		"+15551234567",
		"hello",
		domain.MessageStatusQueued,
		[]byte(`{"id":"wamid.123"}`),
		2,
		createdAt,
		updatedAt,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "messages" WHERE id = $1 ORDER BY "messages"."id" LIMIT $2`)).
		WithArgs(id, 1).
		WillReturnRows(rows)

	msg, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if msg.ID != id {
		t.Fatalf("GetByID() id = %s, want %s", msg.ID, id)
	}
	if msg.Status != domain.MessageStatusQueued {
		t.Fatalf("GetByID() status = %s, want %s", msg.Status, domain.MessageStatusQueued)
	}
	if msg.RetryCount != 2 {
		t.Fatalf("GetByID() retry count = %d, want 2", msg.RetryCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGormMessageRepository_GetByID_NotFound(t *testing.T) {
	db, mock, cleanup := newMockGormDB(t)
	defer cleanup()

	repo := repository.NewMessageRepository(db)
	id := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "messages" WHERE id = $1 ORDER BY "messages"."id" LIMIT $2`)).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"phone_number",
			"body",
			"status",
			"provider_response",
			"retry_count",
			"created_at",
			"updated_at",
		}))

	errMsg, err := repo.GetByID(context.Background(), id)
	if errMsg != nil {
		t.Fatalf("GetByID() message = %v, want nil", errMsg)
	}
	if !errors.Is(err, repository.ErrMessageNotFound) {
		t.Fatalf("GetByID() error = %v, want ErrMessageNotFound", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func newMockGormDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}

	return db, mock, func() {
		_ = sqlDB.Close()
	}
}

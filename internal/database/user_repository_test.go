package database

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mockDB := &mockDatabase{db: sqlxDB}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectExec(`INSERT INTO users`).
			WithArgs(sqlmock.AnyArg(), phone, sqlmock.AnyArg(), "active", false, true, false, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		user, err := repo.CreateUser(phone)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, phone, user.Phone)
		assert.Equal(t, "active", user.Status)
		assert.True(t, user.PhoneVerified)
		assert.False(t, user.ProfileCompleted)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectExec(`INSERT INTO users`).
			WithArgs(sqlmock.AnyArg(), phone, sqlmock.AnyArg(), "active", false, true, false, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(fmt.Errorf("database error"))

		user, err := repo.CreateUser(phone)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "failed to create user")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Duplicate Phone", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectExec(`INSERT INTO users`).
			WithArgs(sqlmock.AnyArg(), phone, sqlmock.AnyArg(), "active", false, true, false, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(fmt.Errorf("duplicate key value violates unique constraint"))

		user, err := repo.CreateUser(phone)
		assert.Error(t, err)
		assert.Nil(t, user)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestGetUserByPhone(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: sqlx.NewDb(db, "sqlmock")}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		phone := "+94712345678"
		userID := uuid.New()
		now := time.Now()

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "email", "first_name", "last_name", "nic",
				"date_of_birth", "address", "city", "postal_code", "gender", "roles",
				"profile_photo_url", "profile_completed", "status",
				"phone_verified", "email_verified", "last_login_at",
				"metadata", "created_at", "updated_at",
			}).AddRow(
				userID, phone, "john@example.com", "John", "Doe", "123456789V",
				now, "123 Main St", "Colombo", "10100", "male", []byte(`{"passenger"}`),
				"http://photo.jpg", true, "active",
				true, true, now,
				"", now, now,
			))

		user, err := repo.GetUserByPhone(phone)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, userID, user.ID)
		assert.Equal(t, phone, user.Phone)
		assert.Equal(t, "John", user.FirstName.String)
		assert.Equal(t, "Doe", user.LastName.String)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetUserByPhone(phone)
		assert.NoError(t, err)
		assert.Nil(t, user)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnError(fmt.Errorf("database error"))

		user, err := repo.GetUserByPhone(phone)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "failed to get user by phone")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestGetUserByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: sqlx.NewDb(db, "sqlmock")}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		phone := "+94712345678"
		now := time.Now()

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "email", "first_name", "last_name", "nic",
				"date_of_birth", "address", "city", "postal_code", "gender", "roles",
				"profile_photo_url", "profile_completed", "status",
				"phone_verified", "email_verified", "last_login_at",
				"metadata", "created_at", "updated_at",
			}).AddRow(
				userID, phone, "jane@example.com", "Jane", "Smith", "987654321V",
				now, "456 Oak Ave", "Kandy", "20000", "female", []byte(`{"passenger","driver"}`),
				"http://photo.jpg", true, "active",
				true, false, now,
				"", now, now,
			))

		user, err := repo.GetUserByID(userID)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, userID, user.ID)
		assert.Equal(t, "Jane", user.FirstName.String)
		assert.Equal(t, "Smith", user.LastName.String)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE id`).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetUserByID(userID)
		assert.NoError(t, err)
		assert.Nil(t, user)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestUpdateProfile(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: sqlx.NewDb(db, "sqlmock")}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		firstName := "John"
		lastName := "Doe"
		email := "john.doe@example.com"
		gender := "male"
		profilePhotoURL := "http://example.com/photo.jpg"
		address := "123 Main Street"
		city := "Colombo"
		postalCode := "10100"
		nic := "199512345678"
		dob := time.Date(1995, 5, 10, 0, 0, 0, 0, time.UTC)

		mock.ExpectExec(`UPDATE users SET`).
			WithArgs(firstName, lastName, email, gender, profilePhotoURL, address, city, postalCode, nic, dob, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateProfile(userID, firstName, lastName, email, gender, profilePhotoURL, address, city, postalCode, nic, &dob)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()
		dob := time.Date(1995, 5, 10, 0, 0, 0, 0, time.UTC)

		mock.ExpectExec(`UPDATE users SET`).
			WithArgs("John", "Doe", "john@example.com", "male", "http://example.com/photo.jpg", "123 Main St", "Colombo", "10100", "199512345678", dob, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.UpdateProfile(userID, "John", "Doe", "john@example.com", "male", "http://example.com/photo.jpg", "123 Main St", "Colombo", "10100", "199512345678", &dob)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		userID := uuid.New()
		dob := time.Date(1995, 5, 10, 0, 0, 0, 0, time.UTC)

		mock.ExpectExec(`UPDATE users SET`).
			WithArgs("John", "Doe", "john@example.com", "male", "http://example.com/photo.jpg", "123 Main St", "Colombo", "10100", "199512345678", dob, sqlmock.AnyArg(), userID).
			WillReturnError(fmt.Errorf("database error"))

		err := repo.UpdateProfile(userID, "John", "Doe", "john@example.com", "male", "http://example.com/photo.jpg", "123 Main St", "Colombo", "10100", "199512345678", &dob)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update profile")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestIsProfileComplete(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: sqlx.NewDb(db, "sqlmock")}
	repo := NewUserRepository(mockDB)

	t.Run("Complete Profile", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT profile_completed FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"profile_completed"}).AddRow(true))

		isComplete, err := repo.IsProfileComplete(userID)
		require.NoError(t, err)
		assert.True(t, isComplete)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Incomplete Profile", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT profile_completed FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"profile_completed"}).AddRow(false))

		isComplete, err := repo.IsProfileComplete(userID)
		require.NoError(t, err)
		assert.False(t, isComplete)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT profile_completed FROM users WHERE id`).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		isComplete, err := repo.IsProfileComplete(userID)
		assert.Error(t, err)
		assert.False(t, isComplete)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestUpdateProfileCompletion(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: sqlx.NewDb(db, "sqlmock")}
	repo := NewUserRepository(mockDB)

	t.Run("Mark Complete", func(t *testing.T) {
		userID := uuid.New()

		// First query to check profile fields
		mock.ExpectQuery(`SELECT first_name, last_name, email, address FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"first_name", "last_name", "email", "address"}).
				AddRow("John", "Doe", "john@example.com", "123 Main St"))

		// Update query to set profile_completed = true
		mock.ExpectExec(`UPDATE users SET profile_completed`).
			WithArgs(true, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateProfileCompletion(userID)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Mark Incomplete - Missing Fields", func(t *testing.T) {
		userID := uuid.New()

		// Query returns incomplete profile (missing email)
		mock.ExpectQuery(`SELECT first_name, last_name, email, address FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"first_name", "last_name", "email", "address"}).
				AddRow("John", "Doe", "", "123 Main St"))

		// Update query to set profile_completed = false
		mock.ExpectExec(`UPDATE users SET profile_completed`).
			WithArgs(false, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateProfileCompletion(userID)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT first_name, last_name, email, address FROM users WHERE id`).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		err := repo.UpdateProfileCompletion(userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestGetOrCreateUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: sqlx.NewDb(db, "sqlmock")}
	repo := NewUserRepository(mockDB)

	t.Run("Get Existing User", func(t *testing.T) {
		phone := "+94712345678"
		userID := uuid.New()
		now := time.Now()

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "email", "first_name", "last_name", "nic",
				"date_of_birth", "address", "city", "postal_code", "gender", "roles",
				"profile_photo_url", "profile_completed", "status",
				"phone_verified", "email_verified", "last_login_at",
				"metadata", "created_at", "updated_at",
			}).AddRow(
				userID, phone, "john@example.com", "John", "Doe", "123456789V",
				now, "123 Main St", "Colombo", "10100", "male", []byte(`{"passenger"}`),
				"http://photo.jpg", true, "active",
				true, true, now,
				"", now, now,
			))

		user, isNew, err := repo.GetOrCreateUser(phone)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.False(t, isNew)
		assert.Equal(t, phone, user.Phone)
		assert.Equal(t, "John", user.FirstName.String)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Create New User", func(t *testing.T) {
		phone := "+94712345678"

		// First query returns no rows (user doesn't exist)
		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnError(sql.ErrNoRows)

		// Then insert new user
		mock.ExpectExec(`INSERT INTO users`).
			WithArgs(sqlmock.AnyArg(), phone, sqlmock.AnyArg(), "active", false, true, false, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		user, isNew, err := repo.GetOrCreateUser(phone)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.True(t, isNew)
		assert.Equal(t, phone, user.Phone)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestUpdateUserStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: sqlx.NewDb(db, "sqlmock")}
	repo := NewUserRepository(mockDB)

	t.Run("Update to Active", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET status`).
			WithArgs("active", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateUserStatus(userID, "active")
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Update to Suspended", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET status`).
			WithArgs("suspended", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateUserStatus(userID, "suspended")
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET status`).
			WithArgs("banned", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.UpdateUserStatus(userID, "banned")
		assert.Error(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestAddUserRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: sqlx.NewDb(db, "sqlmock")}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		role := "driver"

		mock.ExpectExec(`UPDATE users SET roles`).
			WithArgs(role, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.AddUserRole(userID, role)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET roles`).
			WithArgs("admin", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.AddUserRole(userID, "admin")
		assert.Error(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestRemoveUserRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: sqlx.NewDb(db, "sqlmock")}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		role := "driver"

		mock.ExpectExec(`UPDATE users SET roles`).
			WithArgs(role, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.RemoveUserRole(userID, role)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET roles`).
			WithArgs("passenger", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.RemoveUserRole(userID, "passenger")
		assert.Error(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestListUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: sqlx.NewDb(db, "sqlmock")}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		now := time.Now()
		user1ID := uuid.New()
		user2ID := uuid.New()

		mock.ExpectQuery(`SELECT (.+) FROM users ORDER BY created_at DESC LIMIT`).
			WithArgs(10, 0).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "email", "first_name", "last_name", "nic",
				"date_of_birth", "address", "city", "postal_code", "gender", "roles",
				"profile_photo_url", "profile_completed", "status",
				"phone_verified", "email_verified", "last_login_at",
				"metadata", "created_at", "updated_at",
			}).
				AddRow(user1ID, "+94712345678", "john@example.com", "John", "Doe", "123456789V",
					now, "123 Main St", "Colombo", "10100", "male", []byte(`{"passenger"}`),
					"http://photo.jpg", true, "active",
					true, true, now,
					"", now, now).
				AddRow(user2ID, "+94723456789", "jane@example.com", "Jane", "Smith", "987654321V",
					now, "456 Oak Ave", "Kandy", "20000", "female", []byte(`{"passenger","driver"}`),
					"http://photo.jpg", true, "active",
					true, false, now,
					"", now, now))

		users, err := repo.ListUsers(10, 0)
		require.NoError(t, err)
		assert.Len(t, users, 2)
		assert.Equal(t, "John", users[0].FirstName.String)
		assert.Equal(t, "Jane", users[1].FirstName.String)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Empty Result", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM users ORDER BY created_at DESC LIMIT`).
			WithArgs(10, 0).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "email", "first_name", "last_name", "nic",
				"date_of_birth", "address", "city", "postal_code", "gender", "roles",
				"profile_photo_url", "profile_completed", "status",
				"phone_verified", "email_verified", "last_login_at",
				"metadata", "created_at", "updated_at",
			}))

		users, err := repo.ListUsers(10, 0)
		require.NoError(t, err)
		assert.Len(t, users, 0)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM users ORDER BY created_at DESC LIMIT`).
			WithArgs(10, 0).
			WillReturnError(fmt.Errorf("database error"))

		users, err := repo.ListUsers(10, 0)
		assert.Error(t, err)
		assert.Nil(t, users)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestCountUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mockDB := &mockDatabase{db: sqlx.NewDb(db, "sqlmock")}
	repo := NewUserRepository(mockDB)

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

		count, err := repo.CountUsers()
		require.NoError(t, err)
		assert.Equal(t, 42, count)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Zero Users", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		count, err := repo.CountUsers()
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
			WillReturnError(fmt.Errorf("database error"))

		count, err := repo.CountUsers()
		assert.Error(t, err)
		assert.Equal(t, 0, count)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

// Mock database implementation for testing
type mockDatabase struct {
	db *sqlx.DB
}

func (m *mockDatabase) Get(dest interface{}, query string, args ...interface{}) error {
	return m.db.Get(dest, query, args...)
}

func (m *mockDatabase) Select(dest interface{}, query string, args ...interface{}) error {
	return m.db.Select(dest, query, args...)
}

func (m *mockDatabase) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return m.db.Query(query, args...)
}

func (m *mockDatabase) QueryRow(query string, args ...interface{}) *sql.Row {
	return m.db.QueryRow(query, args...)
}

func (m *mockDatabase) Exec(query string, args ...interface{}) (sql.Result, error) {
	return m.db.Exec(query, args...)
}

func (m *mockDatabase) Close() error {
	return m.db.Close()
}

func (m *mockDatabase) Ping() error {
	return m.db.Ping()
}

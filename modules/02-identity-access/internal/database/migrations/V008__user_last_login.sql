-- The user queries in internal/database/user_store.go have always selected
-- last_login_at, and models.User has always carried it, but V001 never created
-- the column. Nothing noticed because M02 ran without a database, so those
-- queries were never executed. Enabling persistence made every user read fail
-- with "column last_login_at does not exist".
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMP WITH TIME ZONE;

-- persistence.WorkflowInstance.Id defaults to uuid_generate_v4() (see
-- gorm:"...;default:uuid_generate_v4()" in persistence/others.go), which needs
-- this extension enabled before AutoMigrate creates the table.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- +goose Up
-- +goose StatementBegin

CREATE TABLE departments (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(64) NOT NULL UNIQUE,
    disabled    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE users (
    id              SERIAL PRIMARY KEY,
    username        VARCHAR(64) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    role            VARCHAR(16) NOT NULL CHECK (role IN ('super','dept','user')),
    department_id   INT REFERENCES departments(id),
    real_name       VARCHAR(64),
    disabled        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_users_role_dept ON users(role, department_id);

CREATE TABLE documents (
    id              SERIAL PRIMARY KEY,
    title           VARCHAR(200) NOT NULL,
    content_html    TEXT NOT NULL,
    publisher_id    INT NOT NULL REFERENCES users(id),
    publisher_dept  INT REFERENCES departments(id),
    target_scope    VARCHAR(16) NOT NULL CHECK (target_scope IN ('DEPARTMENT','ALL_USERS','OWN_DEPARTMENT')),
    deadline        TIMESTAMPTZ,
    status          VARCHAR(16) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','RECALLED')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_documents_publisher ON documents(publisher_id);
CREATE INDEX idx_documents_scope_deadline ON documents(target_scope, deadline);

CREATE TABLE document_targets (
    document_id    INT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    department_id  INT REFERENCES departments(id),
    user_id        INT NOT NULL REFERENCES users(id),
    PRIMARY KEY (document_id, user_id)
);
CREATE INDEX idx_targets_user ON document_targets(user_id);

CREATE TABLE attachments (
    id           SERIAL PRIMARY KEY,
    owner_type   VARCHAR(16) NOT NULL CHECK (owner_type IN ('DOCUMENT','DOCUMENT_DRAFT','SUBMISSION','INLINE')),
    owner_id     INT NOT NULL,
    purpose      VARCHAR(16) CHECK (purpose IS NULL OR purpose = '' OR purpose IN ('READING','TEMPLATE')),
    file_name    VARCHAR(255) NOT NULL,
    stored_path  VARCHAR(500) NOT NULL,
    mime_type    VARCHAR(100) NOT NULL,
    size_bytes   BIGINT NOT NULL,
    uploader_id  INT NOT NULL REFERENCES users(id),
    uploaded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_attachments_owner ON attachments(owner_type, owner_id);

CREATE TABLE submissions (
    id              SERIAL PRIMARY KEY,
    document_id     INT NOT NULL REFERENCES documents(id),
    user_id         INT NOT NULL REFERENCES users(id),
    department_id   INT REFERENCES departments(id),
    current_status  VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                    CHECK (current_status IN ('PENDING','SUBMITTED','SUBMITTED_LATE','RETURNED')),
    submitted_at    TIMESTAMPTZ,
    return_reason   TEXT,
    return_count    INT NOT NULL DEFAULT 0,
    note            TEXT,
    last_action_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (document_id, user_id)
);
CREATE INDEX idx_submissions_doc_status ON submissions(document_id, current_status);
CREATE INDEX idx_submissions_user ON submissions(user_id);
CREATE INDEX idx_submissions_dept ON submissions(department_id);

CREATE TABLE submission_actions (
    id              SERIAL PRIMARY KEY,
    submission_id   INT NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    action_type     VARCHAR(16) NOT NULL CHECK (action_type IN ('SUBMIT','RETURN','RESUBMIT')),
    operator_id     INT NOT NULL REFERENCES users(id),
    reason          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_sub_actions_submission ON submission_actions(submission_id);

CREATE TABLE document_revisions (
    id            SERIAL PRIMARY KEY,
    document_id   INT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    editor_id     INT NOT NULL REFERENCES users(id),
    change_type   VARCHAR(16) NOT NULL CHECK (change_type IN ('CONTENT','ATTACHMENT','DEADLINE','META')),
    diff_summary  TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_doc_revisions_doc ON document_revisions(document_id);

-- 站内通知预留表 (M1 仅建表,UI 在 M3 实现)
CREATE TABLE notifications (
    id          SERIAL PRIMARY KEY,
    user_id     INT NOT NULL REFERENCES users(id),
    kind        VARCHAR(32) NOT NULL,
    payload     JSONB NOT NULL DEFAULT '{}'::jsonb,
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_notifications_user_unread ON notifications(user_id, read_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS document_revisions;
DROP TABLE IF EXISTS submission_actions;
DROP TABLE IF EXISTS submissions;
DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS document_targets;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS departments;
-- +goose StatementEnd

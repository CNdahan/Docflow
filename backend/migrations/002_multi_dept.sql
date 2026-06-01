-- +goose Up
-- +goose StatementBegin

-- 1. 创建用户-部门多对多关联表
CREATE TABLE user_departments (
    user_id       INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    department_id INT NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, department_id)
);
CREATE INDEX idx_user_depts_dept ON user_departments(department_id);

-- 2. 把现有 role='user' 且有部门的记录迁移到关联表
INSERT INTO user_departments (user_id, department_id)
SELECT id, department_id FROM users
WHERE role = 'user' AND department_id IS NOT NULL;

-- 3. role='user' 改名为 role='normal'
-- 先去掉旧约束，加新约束
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = 'normal' WHERE role = 'user';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('super', 'dept', 'normal'));

-- 4. normal 用户的 department_id 清空（关系已迁移到 user_departments）
UPDATE users SET department_id = NULL WHERE role = 'normal';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 恢复 normal 用户的 department_id（取关联表中任意一个）
UPDATE users SET department_id = (
    SELECT department_id FROM user_departments WHERE user_departments.user_id = users.id LIMIT 1
) WHERE role = 'normal';

-- 改回角色名
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = 'user' WHERE role = 'normal';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('super', 'dept', 'user'));

DROP TABLE IF EXISTS user_departments;

-- +goose StatementEnd

-- +goose Up
CREATE TABLE users (
   id VARCHAR(255) NOT NULL,
   name VARCHAR(255) NOT NULL,
   email VARCHAR(255) NOT NULL,
   created_at TIMESTAMP DEFAULT NOW(),
   deleted_at TIMESTAMPTZ,

   CONSTRAINT users_pkey PRIMARY KEY (id),
   CONSTRAINT users_email_key UNIQUE (email)
);

-- +goose Down
DROP TABLE users;
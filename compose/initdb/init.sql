CREATE DATABASE auth;
\c auth
CREATE EXTENSION citus;

SELECT citus_set_coordinator_host(:'c', 5432);
SELECT * FROM citus_add_node(:'w1', 5432);
SELECT * FROM citus_add_node(:'w2', 5432);

CREATE TABLE users (
   id bigserial primary key,
   username varchar(50) UNIQUE,
   password varchar(50),
   firstname varchar(50),
   lastname varchar(50),
   email varchar(50),
   birthdate varchar(50),
   gender varchar(6),
   biography text,
   city varchar(50),
   phone varchar(50) );
INSERT INTO users (username, password, email, phone) VALUES ('admin','admin','admin@admin.com','123456789');

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_users_firstname_trgm
ON users USING gin (firstname gin_trgm_ops);

CREATE INDEX idx_users_lastname_trgm
ON users USING gin (lastname gin_trgm_ops);

CREATE TEMP TABLE users_import (
    full_name TEXT,
    birthdate DATE,
    city TEXT
);

\copy users_import FROM '/people/people.v3.csv' DELIMITER ',' CSV;

INSERT INTO users (
    username,
    password,
    firstname,
    lastname,
    birthdate,
    city
)
SELECT
    lower(split_part(full_name,' ',1)) || substring(md5(random()::text) from 1 for 12)::text,
    substring(md5(random()::text) from 1 for 12) AS password,
    split_part(full_name,' ',2) AS firstname,
    split_part(full_name,' ',1) AS lastname,
    birthdate,
    city
FROM users_import;


CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    username varchar(50) NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT now()
);

CREATE INDEX idx_posts_user_created
ON posts (username, created_at DESC);

CREATE TEMP TABLE posts_import (
    text TEXT
);

\copy posts_import FROM '/people/posts.txt';

WITH users_array AS (
    SELECT array_agg(username) AS users FROM users
)
INSERT INTO posts (
    username,
    text,
    created_at
)
SELECT
    users[(r.val * (array_length(users, 1) - 1) + 1)::int],
    p.text,
    now() - (r.val * interval '30 days')
FROM posts_import p
CROSS JOIN users_array
CROSS JOIN LATERAL (
    SELECT random() + length(p.text)*0 AS val
) r;

CREATE TABLE friends (
    username TEXT NOT NULL,
    friendname TEXT NOT NULL
);

CREATE INDEX idx_friends_user
ON friends(username);

ALTER TABLE friends
ADD CONSTRAINT unique_friend UNIQUE (username, friendname);

CREATE TABLE messages (
    id BIGSERIAL,
    from_user TEXT NOT NULL,
    to_user TEXT NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT now(),
    PRIMARY KEY (from_user, id)
);

CREATE INDEX idx_dialog
ON messages (
    LEAST(from_user, to_user),
    GREATEST(from_user, to_user),
    created_at
);

SELECT create_distributed_table('messages', 'from_user');


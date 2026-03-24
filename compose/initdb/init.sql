CREATE DATABASE auth;
\c auth
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

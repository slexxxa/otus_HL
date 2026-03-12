CREATE DATABASE auth;
\c auth
CREATE TABLE users (
   id bigserial primary key,
   username varchar(20) UNIQUE,
   password varchar(20),
   firstname varchar(20),
   lastname varchar(20),
   email varchar(20),
   birthdate varchar(20),
   gender varchar(20),
   biography varchar(200),
   city varchar(20),
   phone varchar(20) );
INSERT INTO users (username, password) VALUES ('admin','admin');

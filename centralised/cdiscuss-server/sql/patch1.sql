CREATE TABLE users (
  id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
  username VARCHAR (50) UNIQUE NOT NULL,
  salt CHAR(21) NOT NULL,
  pw_hash CHAR(64) NOT NULL, -- sha256
  admin_role BOOL NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_users_username ON users (username);


CREATE TABLE comments (
 id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
 url_hash CHAR(64) NOT NULL, -- sha256
 id_user BIGINT,
 dt_created TIMESTAMP WITHOUT TIME ZONE NOT NULL,
 comment_body TEXT NOT NULL,
 
 CONSTRAINT fk_user
   FOREIGN KEY(id_user) 
   REFERENCES users(id)
   ON DELETE CASCADE -- if account is deleted, than also drop all od deleted user comments
);

CREATE INDEX idx_comments_url_hash ON comments (url_hash);

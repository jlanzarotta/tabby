-- +goose Up
INSERT INTO exclusions (pattern, created_at) VALUES
    ('*token=*', unixepoch()),
    ('*_token*', unixepoch()),
    ('*password*', unixepoch()),
    ('*passwd*', unixepoch()),
    ('*apikey*', unixepoch()),
    ('*api-key*', unixepoch()),
    ('*secret=*', unixepoch()),
    ('*_secret*', unixepoch()),
    ('*_key=*', unixepoch()),
    ('*://*:*@*', unixepoch()),
    ('*authorization:*', unixepoch()),
    ('mysql* -p[^ ]*', unixepoch()),
    ('mariadb* -p[^ ]*', unixepoch()),
    ('sshpass *', unixepoch()),
    ('*docker login* -p*', unixepoch()),
    ('*curl*-u *', unixepoch()),
    ('*curl*--user *', unixepoch())
ON CONFLICT(pattern) DO NOTHING;

DELETE FROM commands
WHERE EXISTS (
    SELECT 1 FROM exclusions
    WHERE lower(commands.command) GLOB lower(exclusions.pattern)
);

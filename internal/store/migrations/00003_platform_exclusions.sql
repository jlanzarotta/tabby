-- +goose Up
INSERT INTO exclusions (pattern, created_at) VALUES
    ('*pwd=*', unixepoch()),
    ('*_pass=*', unixepoch()),
    ('*--pass *', unixepoch()),
    ('*--pass=*', unixepoch()),
    ('*pass:*', unixepoch()),
    ('*passphrase*', unixepoch()),
    ('*-token*', unixepoch()),
    ('*-secret*', unixepoch()),
    ('*private-key*', unixepoch()),
    ('*-creds*', unixepoch()),
    ('*asplaintext*', unixepoch()),
    ('*-----begin*', unixepoch()),
    ('*sqlcmd*-p *', unixepoch()),
    ('*bcp *-p *', unixepoch()),
    ('*sqlpackage*/tp:*', unixepoch()),
    ('*sqlpackage*/sp:*', unixepoch()),
    ('*sqlplus */*', unixepoch()),
    ('*mongosh*-p *', unixepoch()),
    ('*redis-cli*-a *', unixepoch()),
    ('*valkey-cli*-a *', unixepoch()),
    ('*podman login*-p *', unixepoch()),
    ('*buildah login*-p *', unixepoch()),
    ('*skopeo login*-p *', unixepoch()),
    ('*helm registry login*-p *', unixepoch()),
    ('*oc login*-p *', unixepoch()),
    ('*az login*-p *', unixepoch()),
    ('*secret create*', unixepoch()),
    ('*--from-literal*', unixepoch()),
    ('*vault login*', unixepoch()),
    ('*vault kv put*', unixepoch()),
    ('*vault write*', unixepoch()),
    ('*bao login*', unixepoch()),
    ('*bao kv put*', unixepoch()),
    ('*bao write*', unixepoch()),
    ('*rabbitmqctl*add_user*', unixepoch()),
    ('*net use*/user:*', unixepoch()),
    ('*ghp_*', unixepoch()),
    ('*gho_*', unixepoch()),
    ('*github_pat_*', unixepoch()),
    ('*glpat-*', unixepoch()),
    ('*xoxb-*', unixepoch()),
    ('*xoxp-*', unixepoch()),
    ('*sk_live_*', unixepoch())
ON CONFLICT(pattern) DO NOTHING;

DELETE FROM commands
WHERE EXISTS (
    SELECT 1 FROM exclusions
    WHERE lower(commands.command) GLOB lower(exclusions.pattern)
);

CREATE USER postgres_exporter WITH PASSWORD 'password';
GRANT pg_monitor TO postgres_exporter;

GRANT SELECT ON pg_stat_database TO postgres_exporter;

select jsonb_build_object(
    'value', (sum(xact_commit) + sum(xact_rollback))
) as value
from pg_stat_database
where datname = current_database();
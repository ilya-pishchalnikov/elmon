-- elmon get cache hit ratio
select jsonb_build_object(
    'value', round(
        (
            (
                sum(blks_hit) - sum(tup_fetched) 
                / (
                    select setting::numeric
                    from pg_settings
                    where name = 'shared_buffers'
                  )
            )
            / sum(blks_read + blks_hit)
        ) * 100, 
        2
    )
) as value
from pg_stat_database 
where datname = current_database();
with cte_connections_count as (
	select 
		  count (*) filter (where state = 'active') as active_count
		, count (*) filter (where state = 'idle') as idle_count
		, count (*) filter (where state = 'idle in transaction') as idle_tx_count
		, count (*) filter (where state is null) as background_count
	from pg_stat_activity
)
select json_build_object(
    'active', coalesce(active_count, 0),
    'idle', coalesce(idle_count, 0),
    'idle_in_transaction', coalesce(idle_tx_count, 0),
    'background', coalesce(background_count, 0)
) as connection_stats
from cte_connections_count;
select jsonb_build_object('value', count(*)) as value
from pg_stat_activity
where wait_event_type is not null
  and state = 'active';
select 
	  json_build_object('value', count(*)) as value
from pg_locks
where not granted;
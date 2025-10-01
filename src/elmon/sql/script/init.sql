-- Table to store details of monitored database servers
create table if not exists server (
	server_id serial not null,
	environment_name varchar(100) not null,
	name varchar(255) not null,
	host varchar(255) not null,
	port smallint not null,
	host_auth_method varchar(20) null,
	timezone varchar(20),
	ssl_mode varchar(20) null,
	description text null,
	is_active boolean not null,
	created_at timestamptz not null constraint df_server_created_at default (current_timestamp),
	modified_at timestamptz null,
	
	constraint pk_server primary key (server_id),

	constraint uq_server_name unique(name),

	constraint chk_server_port check (port between 1 and 65535),
	constraint chk_server_host_auth_method check (host_auth_method in ('password', 'md5', 'scram-sha-256', 'certificate', 'gss', 'sspi')),
	constraint chk_server_ssl_mode check (ssl_mode in ('disable', 'allow', 'prefer', 'require', 'verify-ca', 'verify-full'))
	
);

-- Table to store encrypted database credentials for servers
create table if not exists credential (
	credential_id serial not null,
	server_id integer not null ,
	encrypted_user_name bytea not null,
	encrypted_password bytea not null,
	description text null,
	is_active boolean not null,
	created_at timestamptz not null constraint df_credential_created_at default (current_timestamp),
	modified_at timestamptz null,

	constraint pk_credential primary key(credential_id),

	constraint fk_credential_server_id foreign key (server_id) references server (server_id)
);

-- Dictionary table for logical groups of metrics
create table if not exists metric_group (
	metric_group_id smallserial not null,
	metric_group_name varchar(255) not null constraint uq_metric_group_metric_group_name unique,
	description text null,

	constraint pk_metric_group primary key (metric_group_id)
);

-- Table defining individual metrics
create table if not exists metric (
	metric_id serial not null,
	metric_group_id smallint not null,
	metric_name varchar(255) not null,
	description text null,

	constraint pk_metric primary key (metric_id),

	constraint fk_metric_metric_group_id foreign key (metric_group_id) references metric_group (metric_group_id),

	constraint uq_metric_metric_name unique (metric_name)
);

-- Main table for storing collected metric values (partitioned by time)
create table if not exists metric_value (
	time timestamptz not null,
	server_id integer not null, -- no foreign key for insert optimization reasons
	metric_id integer not null, -- no foreign key for insert optimization reasons
	metric_value jsonb not null,

	constraint pk_metric_value primary key (server_id, metric_id, time)
) partition by range (time);

-- Function to automatically update the modified_at timestamp column
create or replace function update_modified_at()
returns trigger as $$
begin
	new.modified_at = current_timestamp;
	return new;
end;
$$ language plpgsql;


-- Trigger to execute update_modified_at before updating the server table
create or replace trigger trigger_server_modified_at
	before update on server
	for each row execute function update_modified_at();

-- Trigger to execute update_modified_at before updating the credential table
create or replace trigger trigger_credential_modified_at
	before update on credential
	for each row execute function update_modified_at();

-- Checks if a given text string represents a valid PostgreSQL time zone name 
-- or offset by attempting to set the session's time zone.
create or replace function is_valid_timezone(tz_name text)
returns boolean
language plpgsql
as $$
begin
    execute 'set time zone ' || quote_literal(tz_name);
    return true;
exception
    when invalid_parameter_value then 
        return false;
    when others then
        return false;
end;
$$;

-- Function to validate a timezone string using the custom is_valid_timezone function
create or replace function check_timezone_validity()
returns trigger
language plpgsql
as $$
begin
    if not is_valid_timezone(new.timezone) then
        raise exception 'invalid timezone name: %', new.timezone
        using hint = 'check the name complies with iana format (e.g., europe/berlin or utc).';
    end if;
    
    return new;
end;
$$;

-- Create a trigger that executes the validation function
create or replace trigger trg_check_timezone
before insert or update of timezone on server
for each row
execute function check_timezone_validity();

-- Function to create metric_value partitions for future months
create or replace function create_metric_partition(month_forward integer default 6)
returns void as $$
declare
	partition_date date;
	partition_name text;
	start_date date;
	end_date date;
begin
	-- Create parttiions
	for i in 0..month_forward loop
		partition_date := date_trunc('month', current_date + (i || ' months')::interval);
		partition_name := 'metric_value_' || to_char(partition_date, 'YYYY_MM');
		start_date := partition_date;
		end_date := partition_date + INTERVAL '1 month';
		
		if not exists (select 1 from pg_tables where tablename = partition_name) then
			execute format(
				'create table %i partition of metric_value for values from (%l) to (%l)',
				partition_name,
				start_date,
				end_date
			);
		end if;
	end loop;
end;
$$ language plpgsql;

-- Function to drop old partitions based on retention policy
create or replace function drop_old_metric_partitions(retention_months integer default 6)
returns void as $$
declare
	-- The date up to which partitions should be retained (i.e., everything OLDER than this date will be deleted)
	-- For example, if retention_months = 6 and the current date is 2024-03-15,
	-- old_date will be 2023-09-01. All partitions before 2023-09-01 (i.e., 2023_08, 2023_07, etc.) will be deleted.
	retention_cutoff_date date;
	
	-- Variable to store the name of the partition to be dropped
	partition_to_drop text;
begin
	-- Calculate the cutoff date: the start of the month that is 'retention_months' back from the current month.
	retention_cutoff_date := date_trunc('month', current_date - (retention_months || ' months')::interval);

	raise notice 'Retention cutoff date: % (Partitions older than this will be dropped)', retention_cutoff_date;

	-- Iterate through all partitions that are older than retention_cutoff_date
	for partition_to_drop in (
		select relid::regclass::text -- Get the full table name of the partition
		from pg_catalog.pg_inherits pi
		join pg_catalog.pg_class pc on pi.inhrelid = pc.oid
		join pg_catalog.pg_namespace pn on pc.relnamespace = pn.oid
		-- Find partitions whose names start with 'metric_value_'
		where pn.nspname = current_schema() -- Assuming partitions are in the current schema
			and pc.relname like 'metric_value_%'
			-- Extract the date from the partition name and compare it with retention_cutoff_date
			and to_date(substring(pc.relname from 'metric_value_(\d{4}_\d{2})'), 'YYYY_MM') < retention_cutoff_date
		order by to_date(substring(pc.relname from 'metric_value_(\d{4}_\d{2})'), 'YYYY_MM') asc
	)
	loop
		raise notice 'Dropping partition: %', partition_to_drop;
		execute format('DROP TABLE IF EXISTS %I', partition_to_drop);
	end loop;

	raise notice 'Finished dropping old metric partitions.';
end;
$$ language plpgsql;
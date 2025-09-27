create table if not exists environment (
    environment_id smallserial not null,
    environment_name varchar(100) not null constraint uq_environment_environment_name unique,
    description text null,

    constraint pk_environment primary key (environment_id)
);

create table if not exists host_auth_method (
    host_auth_method_id smallserial not null,
    host_auth_method_name varchar(50) not null constraint uq_host_auth_method_host_auth_method_name unique,

    constraint pk_host_auth_method primary key (host_auth_method_id)
);

create table if not exists ssl_mode (
    ssl_mode_id smallserial not null,
    ssl_mode_name varchar(50) not null constraint uq_ssl_mode_ssl_mode_name unique,

    constraint pk_ssl_mode primary key (ssl_mode_id)
);

create table if not exists timezone (
    timezone_id smallserial not null,
    timezone_name varchar(50) not null constraint uq_timezone_timezone_name unique,

    constraint pk_timezone primary key (timezone_id)
);
create table if not exists server (
    server_id serial not null,
    environment_id smallint not null,
    name varchar(255) not null,
    host varchar(255) not null,
    port smallint not null constraint chk_server_port check (port between 1 and 65535),
    host_auth_method_id smallint null,
    timezone_id smallint null,
    ssl_mode_id smallint null,
    description text null,
    is_active boolean not null,
    created_at timestamptz constraint df_server_created_at default (current_timestamp) not null,
    modified_at timestamptz null,
    
    constraint pk_server primary key (server_id),
    
    constraint fk_server_environment_id foreign key (environment_id) references environment(environment_id),
    constraint fk_server_host_auth_method_id foreign key (host_auth_method_id) references host_auth_method(host_auth_method_id),
    constraint fk_server_timezone_id foreign key (timezone_id) references timezone(timezone_id),
    constraint fk_server_ssl_mode_id foreign key (ssl_mode_id) references ssl_mode(ssl_mode_id)
);

create table if not exists credential (
    credential_id serial not null,
    server_id integer not null ,
    encrypted_user_name bytea not null,
    encrypted_password bytea not null,
    description text null,
    is_active boolean not null,
    created_at timestamptz not null constraint df_credential_created_at default (current_timestamp),
    modified_at timestamptz null,

    constraint pk_credential  primary key (credential_id),

    constraint fk_credential_server_id foreign key (server_id) references server (server_id)    
);

create table if not exists metric_group (
    metric_group_id smallserial not null,
    metric_group_name varchar(255) not null constraint uq_metric_group_metric_group_name unique,
    description text null,

    constraint pk_metric_group primary key (metric_group_id)
);

create table if not exists metric (
    metric_id serial not null,
    metric_group_id smallint not null,
    metric_name varchar(255) not null,
    description text null,

    constraint pk_metric primary key (metric_id),

    constraint fk_metric_metric_group_id foreign key (metric_group_id) references metric_group (metric_group_id)
);

create table if not exists metric_value (
    time timestamptz not null,
    server_id integer not null, -- no foreign key for insert optimization reasons
    metric_id integer not null, -- no foreign key for insert optimization reasons
    metric_value jsonb not null,

    constraint pk_metric_value primary key (server_id, metric_id, time)
) partition by range (time);

-- function for modified_at update triggers
create or replace function update_modified_at()
returns trigger as $$
begin
    new.modified_at = current_timestamp;
    return new;
end;
$$ language plpgsql;

create or replace trigger trigger_server_modified_at
    before update on server
    for each row execute function update_modified_at();

create or replace trigger trigger_credential_modified_at
    before update on credential
    for each row execute function update_modified_at();

-- Create metric partitions function
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

-- Drop old metric partitions function
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

-- fill dictionaries
insert into host_auth_method (host_auth_method_name) values 
    ('password'), ('md5'), ('scram-sha-256'), ('certificate'), ('gss'), ('sspi')
on conflict (host_auth_method_name) do nothing;

insert into ssl_mode (ssl_mode_name) values 
    ('disable'), ('allow'), ('prefer'), ('require'), ('verify-ca'), ('verify-full')
on conflict (ssl_mode_name) do nothing;


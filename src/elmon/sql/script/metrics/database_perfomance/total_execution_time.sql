WITH aggregated_data AS (
    SELECT 
        query_type,
        SUM(calls) as execution_count,
        SUM(total_exec_time) as total_time_ms
    FROM (
        SELECT 
            CASE 
                WHEN query ~* '^SELECT' THEN 'SELECT'
                WHEN query ~* '^INSERT' THEN 'INSERT'
                WHEN query ~* '^UPDATE' THEN 'UPDATE'
                WHEN query ~* '^DELETE' THEN 'DELETE'
                WHEN query ~* '^CREATE' THEN 'DDL'
                WHEN query ~* '^ALTER' THEN 'DDL'
                WHEN query ~* '^DROP' THEN 'DDL'
                ELSE 'OTHER'
            END as query_type,
            total_exec_time,
            calls
        FROM pg_stat_statements
        WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
    ) as typed_queries
    GROUP BY query_type
),
json_data AS (
    SELECT 
        json_build_object(
            'execution_count', execution_count,
            'total_time_ms', total_time_ms
        ) as stats,
        query_type
    FROM aggregated_data
)
SELECT json_build_object(
    'select', COALESCE((SELECT stats FROM json_data WHERE query_type = 'SELECT'), '{"execution_count": 0, "total_time_ms": 0}'::json),
    'insert', COALESCE((SELECT stats FROM json_data WHERE query_type = 'INSERT'), '{"execution_count": 0, "total_time_ms": 0}'::json),
    'update', COALESCE((SELECT stats FROM json_data WHERE query_type = 'UPDATE'), '{"execution_count": 0, "total_time_ms": 0}'::json),
    'delete', COALESCE((SELECT stats FROM json_data WHERE query_type = 'DELETE'), '{"execution_count": 0, "total_time_ms": 0}'::json),
    'ddl', COALESCE((SELECT stats FROM json_data WHERE query_type = 'DDL'), '{"execution_count": 0, "total_time_ms": 0}'::json),
    'other', COALESCE((SELECT stats FROM json_data WHERE query_type = 'OTHER'), '{"execution_count": 0, "total_time_ms": 0}'::json)
) as query_stats;
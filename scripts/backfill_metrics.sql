-- Backfill model_metrics table from existing usage_logs data
-- This script creates historical metrics data for the model health heatmap

-- First, create hourly model metrics for the last 30 days
INSERT INTO model_metrics (id, model_name, interval, timestamp, health_score, avg_latency, p95_latency, p99_latency, total_requests, failed_requests, success_rate, total_tokens, input_tokens, output_tokens, total_cost, circuit_open, circuit_failures, created_at, updated_at)
SELECT 
    gen_random_uuid() as id,
    model as model_name,
    'hourly' as interval,
    date_trunc('hour', created_at) as timestamp,
    CASE 
        WHEN COUNT(*) = 0 THEN 0
        WHEN SUM(CASE WHEN error IS NOT NULL AND error != '' THEN 1 ELSE 0 END)::float / COUNT(*) > 0.1 THEN 50  -- High error rate
        WHEN AVG(latency) > 10000 THEN 60  -- Slow responses
        WHEN AVG(latency) > 5000 THEN 80   -- Moderate latency
        ELSE 95  -- Good health
    END as health_score,
    COALESCE(AVG(latency), 0)::bigint as avg_latency,
    COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency), 0)::bigint as p95_latency,
    COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency), 0)::bigint as p99_latency,
    COUNT(*) as total_requests,
    SUM(CASE WHEN error IS NOT NULL AND error != '' THEN 1 ELSE 0 END) as failed_requests,
    CASE 
        WHEN COUNT(*) = 0 THEN 0
        ELSE (100.0 - (SUM(CASE WHEN error IS NOT NULL AND error != '' THEN 1 ELSE 0 END)::float / COUNT(*) * 100))
    END as success_rate,
    SUM(COALESCE(total_tokens, 0)) as total_tokens,
    SUM(COALESCE(input_tokens, 0)) as input_tokens,
    SUM(COALESCE(output_tokens, 0)) as output_tokens,
    SUM(COALESCE(total_cost, 0)) as total_cost,
    false as circuit_open,  -- Assume circuits are not open for historical data
    SUM(CASE WHEN error IS NOT NULL AND error != '' THEN 1 ELSE 0 END) as circuit_failures,
    NOW() as created_at,
    NOW() as updated_at
FROM usage_logs 
WHERE created_at >= NOW() - INTERVAL '30 days'
  AND model IS NOT NULL
  AND model != ''
GROUP BY model, date_trunc('hour', created_at)
ON CONFLICT DO NOTHING;  -- Don't overwrite existing data

-- Create daily model metrics by aggregating hourly data
INSERT INTO model_metrics (id, model_name, interval, timestamp, health_score, avg_latency, p95_latency, p99_latency, total_requests, failed_requests, success_rate, total_tokens, input_tokens, output_tokens, total_cost, circuit_open, circuit_failures, created_at, updated_at)
SELECT 
    gen_random_uuid() as id,
    model as model_name,
    'daily' as interval,
    date_trunc('day', created_at) as timestamp,
    CASE 
        WHEN COUNT(*) = 0 THEN 0
        WHEN SUM(CASE WHEN error IS NOT NULL AND error != '' THEN 1 ELSE 0 END)::float / COUNT(*) > 0.1 THEN 50
        WHEN AVG(latency) > 10000 THEN 60
        WHEN AVG(latency) > 5000 THEN 80
        ELSE 95
    END as health_score,
    COALESCE(AVG(latency), 0)::bigint as avg_latency,
    COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency), 0)::bigint as p95_latency,
    COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency), 0)::bigint as p99_latency,
    COUNT(*) as total_requests,
    SUM(CASE WHEN error IS NOT NULL AND error != '' THEN 1 ELSE 0 END) as failed_requests,
    CASE 
        WHEN COUNT(*) = 0 THEN 0
        ELSE (100.0 - (SUM(CASE WHEN error IS NOT NULL AND error != '' THEN 1 ELSE 0 END)::float / COUNT(*) * 100))
    END as success_rate,
    SUM(COALESCE(total_tokens, 0)) as total_tokens,
    SUM(COALESCE(input_tokens, 0)) as input_tokens,
    SUM(COALESCE(output_tokens, 0)) as output_tokens,
    SUM(COALESCE(total_cost, 0)) as total_cost,
    false as circuit_open,
    SUM(CASE WHEN error IS NOT NULL AND error != '' THEN 1 ELSE 0 END) as circuit_failures,
    NOW() as created_at,
    NOW() as updated_at
FROM usage_logs 
WHERE created_at >= NOW() - INTERVAL '30 days'
  AND model IS NOT NULL
  AND model != ''
GROUP BY model, date_trunc('day', created_at)
ON CONFLICT DO NOTHING;

-- Show summary of what was created
SELECT 
    'Summary of backfilled data:' as message,
    COUNT(*) as total_records,
    COUNT(DISTINCT model_name) as unique_models,
    MIN(timestamp) as earliest_date,
    MAX(timestamp) as latest_date
FROM model_metrics;
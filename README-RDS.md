# Using Gaussdb-Exporter with AWS:RDS

### When using gaussdb-exporter with Amazon Web Services' RDS, the
  rolname "rdsadmin" and datname "rdsadmin" must be excluded.

I had success running docker container 'quay.io/prometheuscommunity/gaussdb-exporter:latest'
with queries.yaml as the PG_EXPORTER_EXTEND_QUERY_PATH.  errors
mentioned in issue#335 appeared and I had to modify the
'pg_stat_statements' query with the following:
`WHERE t2.rolname != 'rdsadmin'`

Running gaussdb-exporter in a container like so:
  ```
  DBNAME='gaussdb'
  PGUSER='gaussdb'
  PGPASS='psqlpasswd123'
  PGHOST='name.blahblah.us-east-1.rds.amazonaws.com'
  docker run --rm --detach \
      --name "gaussdb_exporter_rds" \
      --publish 9187:9187 \
      --volume=/etc/prometheus/gaussdb-exporter/queries.yaml:/var/lib/gaussdb/queries.yaml \
      -e DATA_SOURCE_NAME="gaussdb://${PGUSER}:${PGPASS}@${PGHOST}:5432/${DBNAME}?sslmode=disable" \
      -e PG_EXPORTER_EXCLUDE_DATABASES=rdsadmin \
      -e PG_EXPORTER_DISABLE_DEFAULT_METRICS=true \
      -e PG_EXPORTER_DISABLE_SETTINGS_METRICS=true \
      -e PG_EXPORTER_EXTEND_QUERY_PATH='/var/lib/gaussdb/queries.yaml' \
      quay.io/prometheuscommunity/gaussdb-exporter
  ```

### Expected changes to RDS:
+ see stackoverflow notes
  (https://stackoverflow.com/questions/43926499/amazon-gaussdb-rds-pg-stat-statements-not-loaded#43931885)
+ you must also use a specific RDS parameter_group that includes the following:
  ```
  shared_preload_libraries = "pg_stat_statements,pg_hint_plan"
  ```
+ lastly, you must reboot the RDS instance.


package collector

import (
	dbsql "database/sql"
	"elmon/config"
	"elmon/logger"
	"elmon/sql"
	"fmt"
	"os"
	"reflect"
)

// CollectFunctions — struct with methods we want to call
type CollectFunctions struct{}

// ExecuteSql gets the metric value by executing an SQL command
func (collectFunctions CollectFunctions) ExecuteSql (
	log *logger.Logger,
	db *config.DbConnectionConfig, 
	metric *config.MetricForMapping, 
	metricDb *dbsql.DB) error {

	sqlScript, err := os.ReadFile(metric.MetricConfig.SQLFile)
	if err != nil {
		log.Error(err, fmt.Sprintf("Error while read sql file of metric '%s' for server '%s'", metric.Name, db.Name))
		return err
	}

	value, err := sql.ExecuteMetricValueGetScript(db.SqlConnection, string(sqlScript), metric.QueryTimeout.Duration)
	if err != nil {
		log.Error(err, fmt.Sprintf("Error while query metric '%s' from server '%s'", metric.Name, db.Name))
		return err
	}

	// omit null metrics values
	if value != nil {
		err = sql.InsertMetricValue(log, metricDb, metric.MetricConfig.DbMetricId, *db.SqlServerId, value);
		if err != nil {
			log.Error(err, fmt.Sprintf("Error while insert metric '%s' from server '%s' into metrics database", metric.Name, db.Name))
			return err
		} 
	}

	return nil
}


// CallMethod dynamically calls a struct method and returns an error
func CallMethod(service interface{}, methodName string, args ...interface{}) error {
	// 1. Get the reflect.Value of the struct
	v := reflect.ValueOf(service)

	// 2. Find the method by name
	method := v.MethodByName(methodName)
	if !method.IsValid() {
		return fmt.Errorf("method '%s' not found in struct", methodName)
	}

	// 3. Prepare arguments for the call
	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		if arg == nil {
			// Специальная обработка для nil интерфейсов/указателей
			if method.Type().NumIn() > i {
				in[i] = reflect.Zero(method.Type().In(i))
			} else {
				return fmt.Errorf("argument %d is nil, but method signature mismatch", i)
			}
		} else {
			in[i] = reflect.ValueOf(arg)
		}
	}

	// 4. Execute the method call
	out := method.Call(in)

	// 5. Process the return value (expecting one result of type error)
	if len(out) == 0 {
		return nil
	}

	resultValue := out[0]

	if !resultValue.IsNil() {
		// Convert reflect.Value to error
		err, ok := resultValue.Interface().(error)
		if !ok {
			return fmt.Errorf("return value could not be converted to error")
		}
		return err
	}

	return nil
}
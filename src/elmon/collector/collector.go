package collector

import (
	// dbsql "database/sql"
	// "elmon/sql"
	// "elmon/config"
	// "elmon/logger"
	"fmt"
	// "os"
	"reflect"
)

// MyService — структура с методами, которые мы хотим вызывать
type CollectFunctions struct{}

// // 
// func (collectFunctions CollectFunctions) ExecuteSql (
// 	log *logger.Logger,
// 	db *config.DbConnectionConfig, 
// 	metric *config.MetricForMapping, 
// 	metricDb *dbsql.DB) error {

// 		sqlScript, err := os.ReadFile(metric.MetricConfig.SQLFile)
// 		if err != nil {
// 			log.Error(err, fmt.Sprintf("Error while read sql file of metric '%s' for server '%s'", metric.Name, db.Name))
// 			return err
// 		}

// 		value, err = sql.ExecuteMetricValueGetScriptWithTimeout(db.SqlConnection, string(sqlScript), metric.QueryTimeout.Duration)
// 		if err != nil {
// 			log.Error(err, fmt.Sprintf("Error while query metric '%s' from server '%s'", metric.Name, db.Name))
// 			return err
// 		}

// 		err = sql.InsertMetricValue(log, db.SqlConnection, metric.MetricConfig.DbMetricId, )


// 	return nil
// }

// ----------------------------------------------------------------------------------

// CallMethodAndReturnError динамически вызывает метод структуры и возвращает error
//
// service: экземпляр структуры, содержащий метод.
// methodName: строковое имя вызываемого метода.
// param: параметр, передаваемый методу.
func CallMethodAndReturnError(service interface{}, methodName string, param interface{}) error {
	// 1. Получаем reflect.Value структуры
	v := reflect.ValueOf(service)

	// 2. Находим метод по имени
	method := v.MethodByName(methodName)
	if !method.IsValid() {
		return fmt.Errorf("метод '%s' не найден в структуре", methodName)
	}

	// 3. Проверяем сигнатуру метода (ожидаем один входной параметр и один выходной error)
	methodType := method.Type()
	if methodType.NumIn() != 1 || methodType.NumOut() != 1 {
		return fmt.Errorf("метод '%s' должен принимать 1 параметр и возвращать 1 результат", methodName)
	}

	// 4. Проверяем тип возвращаемого значения (должен быть error)
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	if methodType.Out(0) != errorType {
		return fmt.Errorf("метод '%s' должен возвращать тип error, а возвращает %s", methodName, methodType.Out(0))
	}

	// 5. Подготавливаем аргументы для вызова
	// Создаем []reflect.Value с нашим единственным параметром
	in := []reflect.Value{reflect.ValueOf(param)}

	// 6. Выполняем вызов метода
	out := method.Call(in)

	// 7. Обрабатываем возвращаемое значение (ожидаем один результат типа error)
	if len(out) == 0 {
		return nil // Теоретически не должно случиться из-за проверки NumOut, но для безопасности
	}

	// Получаем первый (и единственный) результат
	resultValue := out[0]

	// Если возвращаемое значение не nil
	if !resultValue.IsNil() {
		// Преобразуем reflect.Value в interface{} и затем в error
		err, ok := resultValue.Interface().(error)
		if !ok {
			// Это должно быть невозможно из-за проверки Out(0), но на всякий случай
			return fmt.Errorf("возвращаемое значение не удалось преобразовать в error")
		}
		return err
	}

	// Возвращаемое значение nil (нет ошибки)
	return nil
}
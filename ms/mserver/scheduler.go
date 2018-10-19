package mserver

import (
	"expvar"
	"fmt"
	"ms/common"
	customLog "ms/log"
	"ms/redis"
	"strconv"
	"time"
)

var (
	numberOfOrders                 = expvar.NewInt("number_of_incoming_service_orders")
	numberOfOrdersMovedToList      = expvar.NewInt("number_of_successful_order_list_inserts")
	numberOfFailedRedisZSETInserts = expvar.NewInt("number_of_failed_order_ZSET_inserts")
	numberOfFailedRedisListInserts = expvar.NewInt("number_of_failed_order_list_inserts")
)

/*
The package contains functions needed for the actual service, including the Service HandlerFunc
That allows to only make changes to this file and not the actual controller that servs customer
requests overal
*/

// StoreNewOrderInRedis function is used to add a new "order" in the Redis ZSET with
// the score that is a dd (due date, unix timestamp value of when to process the order)
func StoreNewOrderInRedis(zsetName, dd, orderID string, redisClient *redis.Client, logger customLog.LevelLogger) error {
	numberOfOrders.Add(1)

	setNameRedisStr := redis.NewBulkStr(zsetName)
	ddRedisStr := redis.NewBulkStr(dd)
	orderIDRedisStr := redis.NewBulkStr(orderID)
	command := redis.NewCommand(redis.ZADD, []redis.Marshaller{setNameRedisStr, ddRedisStr, orderIDRedisStr}).Marshal()
	logger.Trace.Printf("scheduler: command to save a new order in incoming set with score of %v constructed: \n%v", ddRedisStr, command)

	return redisClient.Send(command)
}

// StartScheduler starts the scheduler process as a goroutine
func StartScheduler(config *common.Config, logger customLog.LevelLogger) error {
	ch := make(chan redis.Marshaller, 5)
	errCh := make(chan error)
	go dueDateOrders(ch, errCh, config, logger)

	for {
		select {
		case order := <-ch:
			moveErr := moveDDOrderToProcessingQueue(order, logger, config)
			if moveErr != nil {
				numberOfFailedRedisListInserts.Add(1)
				logger.Error.Printf("scheduler: error updating the order list %v with order %v: %v\n", config.Service.OrderListName, order, moveErr)
			}
		case err := <-errCh:
			return err
		}
	}
}

// func moveDDOrderToProcessingQueue moves the orders that reached DD into the list
func moveDDOrderToProcessingQueue(order redis.Marshaller, logger customLog.LevelLogger, config *common.Config) error {
	logger.Trace.Printf("scheduler: attempting to connect to the Redis server %v:%v\n", config.Service.RedisHost, config.Service.RedisPort)
	redisClient, redisErr := redis.Connect(config.Service.RedisHost, config.Service.RedisPort)
	if redisErr != nil {
		redisClient.Conn.Close()
		return redisErr
	}

	listName := redis.NewBulkStr(config.Service.OrderListName)
	command := redis.NewCommand(redis.RPUSH, []redis.Marshaller{listName, order}).Marshal()
	logger.Trace.Printf("scheduler: command created to send the order to the processing list: \n%v", command)

	err := redisClient.Send(command)
	if err != nil {
		return err
	}
	logger.Trace.Printf("scheduler: order list updated successfully\n")

	zsetName := redis.NewBulkStr(config.Service.OrderSetName)
	command = redis.NewCommand(redis.ZREM, []redis.Marshaller{zsetName, order}).Marshal()
	logger.Trace.Printf("scheduler: command to remove order %v from the %v set constructed:\n %v", order, config.Service.OrderSetName, command)
	err = redisClient.Send(command)
	if err != nil {
		return err
	}
	logger.Trace.Printf("scheduler ZSet updated successfully\n")
	numberOfOrdersMovedToList.Add(1)

	return nil
}

// dueDateOrders checks the Redis and POPs the topmost order that reached the Due Date
// if there is one, it will be passed in the channel, if no orders are ready to be worked, the func will sleep and do nothing
func dueDateOrders(ch chan redis.Marshaller, errCh chan error, config *common.Config, logger customLog.LevelLogger) {
	stStr := redis.NewBulkStr("0")
	zName := redis.NewBulkStr(config.Service.OrderSetName)

	for {
		logger.Trace.Printf("scheduler: attempting to connect to the Redis server %v:%v\n", config.Service.RedisHost, config.Service.RedisPort)
		redisClient, redisErr := redis.Connect(config.Service.RedisHost, config.Service.RedisPort)
		if redisErr != nil {
			redisClient.Conn.Close()
			numberOfFailedRedisZSETInserts.Add(1)
			errCh <- fmt.Errorf("scheduler: Redis client is unreachable: %v", redisErr)
		}
		defer redisClient.Conn.Close()

		unixTS := strconv.FormatInt(time.Now().Unix(), 10)
		endStr := redis.NewBulkStr(unixTS)
		com := redis.NewCommand(redis.ZANGEBYSCORE, []redis.Marshaller{zName, stStr, endStr})
		logger.Trace.Printf("scheduler: checking for orders with due date before %v\n", unixTS)

		data, err := redisClient.SendReceive(com.Marshal())
		if err != nil {
			numberOfFailedRedisZSETInserts.Add(1)
			logger.Error.Printf("scheduler: call to get the list of orders to process failed: %v\n", err)
		}

		logger.Trace.Printf("scheduler: the call output is: \n%v", string(data[:]))
		object, err := unmarshalTheResponse(data)
		if err != nil {
			numberOfFailedRedisZSETInserts.Add(1)
			logger.Error.Printf("scheduler: call to get the list of orders to process failed: %v\n", err)
		}

		for _, elem := range object.(redis.ArrayType).Value {
			ch <- elem
		}

		time.Sleep(time.Duration(config.Service.WaitTime) * time.Second)

	}
}

func unmarshalTheResponse(data []byte) (redis.Marshaller, error) {
	val, err := redis.Unmarshal(data)
	return val, err

}

package redis

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// The file contains definition of the RESP protocol used by redis server to communicate
// the protocol defines the following data types:
// 1. Simple string: the first byte in response is '+', such as '+OK\r\n'
// 2. Error: the first byte is '-', such as '-Error message\r\n'
// 3. Integer: the first byte is ':', for example, ':42\r\n
// 4. Bulk String: the first byte is '$', then the number of bytes in string, e.g. '$6\r\nsimple\r\n
// 5. Array: the first byte is '*' followed by the number of elements, example '*3\r\n:1\r\n:2\r\n:3\r\n'
// The NULL could be represented in Array and Bulk String ('$-1\r\n)
// Different parts of protocol are terminated with \r\n (CRLF)

// Prefix is a basic type for the RESP prefix implementations
type Prefix string

func (p Prefix) toString() string {
	return string(p)
}

// CommandStr describes the Command in the Redis
type CommandStr string

func (c CommandStr) toString() string {
	return string(c)
}

// List of the commands supported by the Redis server
// Not all of them are listed - add as needed
const (
	// Commands for the Strings

	// STRLEN Length of the String
	STRLEN CommandStr = "STRLEN"
	// APPEND key value a value to the key
	APPEND CommandStr = "APPEND"
	// BITCOUNT key [start end] the number of bits in the String
	BITCOUNT CommandStr = "BITCOUNT"
	// GET key the value of the key
	GET CommandStr = "GET"
	// BITFIELD key [GET type offset] [SET type offset value] [INCRBY type offset increment] [OVERFLOW WRAP|SAT|FAIL] tream str as a
	//array of bits and address specific bits on it
	BITFIELD CommandStr = "BITFIELD"
	// DECR key decrement integer value of a key
	DECR CommandStr = "DECR"
	// INCR key increment an int value of the key
	INCR CommandStr = "INCR"
	// DECRBY key decrement: decrement by X
	DECRBY CommandStr = "DECRBY"
	// INCRBY key increment: increment key by X
	INCRBY CommandStr = "INCRBY"
	// GETSET key value: SET the value on the key and return the old value
	GETSET CommandStr = "GETSET"
	// GETRANGE key start end: substring of the string
	GETRANGE CommandStr = "GETRANGE"
	// MGET key [key ...] to get the value of the given keys
	MGET CommandStr = "MGET"

	// Commands for the ZSET (sorted set)

	// BZPOPMIN key [key ...] timeout: POP the member for list(s) with the lowest score or block until one is available
	BZPOPMIN CommandStr = "BZPOPMIN"
	// BZPOPMAX key [key ...] timeout: same as above, but for Max score
	BZPOPMAX CommandStr = "BZPOPMAX"
	// ZADD key [MX|XX] [CH] [INCR] score: add one of more members to the sorted set or update the score that already exists
	ZADD CommandStr = "ZADD"
	// ZREM key member [member ...] removes an element from the set
	ZREM CommandStr = "ZREM"
	// ZCARD key: get the number of members in a set
	ZCARD CommandStr = "ZCARD"
	// ZCOUNT key min max: count the members in the set within the given range of scores
	ZCOUNT CommandStr = "ZCOUNT"
	// ZINCRBY key increment value: increment the score of a member in a set
	ZINCRBY CommandStr = "ZINCRBY"
	// ZINTERSTORE destination numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM}MIN|MAX] intersect the sets
	// and store the new one in a new key
	ZINTERSTORE CommandStr = "ZINTERSTORE"
	// ZPOPMAX key [count] remove and return members with the MAX score
	ZPOPMAX CommandStr = "ZPOPMAX"
	// ZPOPMIN key [count] remove and return members with the MIN score
	ZPOPMIN CommandStr = "ZPOPMIN"
	// ZRANGE key start stop [WITHSCORES]: returns the list of items that are within the score range
	ZRANGE CommandStr = "ZRANGE"
	// ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]: returns the elements in the set in order by score
	ZANGEBYSCORE CommandStr = "ZRANGEBYSCORE"
	// ZLEXCOUNT key min max: count the number of items in set in lexographical order
	ZLEXCOUNT CommandStr = "ZLEXCOUNT"
	// ZRANGEBYLEX key min max [LIMIT offset count]: returns the range of members in Lex order
	ZRANGEBYLEX CommandStr = "ZRANGEBYLEX"
	// ZREVRANGEBYLEX key max min [LIMIT offset count]: same as above by in reverse order
	ZREVRANGEBYLEX CommandStr = "ZREVRANGEBYLEX"
	// ZRANK key member used to check the index of a member in a set
	ZRANK CommandStr = "ZRANK"
	// ZREVRANGE key start stop [WITHSCORES] returns a range of elements by score in reverse order
	ZREVRANGE CommandStr = "ZREVRANGE"
	// ZSCORE key member get a score of the member in the list
	ZSCORE CommandStr = "ZSCORE"
	// ZSCAN key cursor [MATCH pattern] [COUNT count] iterate elements and scores of the set
	ZSCAN CommandStr = "ZSCAN"
	// ZUNIONSCORE destination numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX]: combine multiple sets into the resulting one
	ZUNIONSCORE CommandStr = "ZUNIONSCORE"

	// Commands for the List

	// BLPOP key [key ...] timeout: remove and get the first element in the List
	BLPOP CommandStr = "BLPOP"
	// BRPOP key [key ...] timeout: same as above but for the last element
	BRPOP CommandStr = "BRPOP"
	// BRPOPPUSH source destination timeout: pop a value from a list and push to another list
	BRPOPPUSH CommandStr = "BRPOPPUSH"
	// LINDEX key index: get an element by index
	LINDEX CommandStr = "LINDEX"
	// LINSERT key BEFORE|AFTER pivot value: insert element before or after another element
	LINSERT CommandStr = "LINSERT"
	// LLEN key returns length of the list
	LLEN CommandStr = "LLEN"
	// LPOP key: get and remove the first element
	LPOP CommandStr = "LPOP"
	// LPUSH key value [value ...] prepend one or many values to the list
	LPUSH CommandStr = "LPUSH"
	// LPUSHX key value pushes a value to the list only if the list exists
	LPUSHX CommandStr = "LPUSHX"
	// LRANGE key start stop retuns sublist
	LRANGE CommandStr = "LRANGE"
	// LREM key count value: removes elements from the list
	LREM CommandStr = "LREM"
	// LSET key index value: set the value of the element at a given index
	LSET CommandStr = "LSET"
	// LTRIM key start stop: trim the list to the specified range
	LTRIM CommandStr = "LTRIM"
	// RPOP key removes and returns the last element on the list
	RPOP CommandStr = "RPOP"
	// RPOPPUSH source destination: removes the last element
	RPOPPUSH CommandStr = "RPOPPUSH"
	// RPUSH key value [value ...] append one or many items to the list
	RPUSH CommandStr = "RPUSH"
	// RPUSHX key value [value ...] same as above but only if the list exists
	RPUSHX CommandStr = "RPUSHX"
)

const (
	// SimplePrefix defines the prefix to be used for the Simple String serialization in Redis
	SimplePrefix Prefix = "+"

	// ErrorPrefix defines the prefix to be used for the Error serialization in Redis
	ErrorPrefix Prefix = "-"

	// IntegerPrefix defines the prefix to be used for the Integer serialization in Redis
	IntegerPrefix Prefix = ":"

	// BulkStringPrefix defines the prefix to be used for the Bulk String serialization in Redis
	BulkStringPrefix Prefix = "$"

	// ArrayPrefix defines the prefix to be used for the Simple String serialization in Redis
	ArrayPrefix Prefix = "*"

	//Separator is used to separate parts of the statement in the Command/Response of the protocol
	Separator string = "\r\n"
)

// Type is basic structure that is used to define all the Types in redis
type Type struct {
	prefix Prefix
}

// Marshaller interface defines the methods that need to be implemetned by each Redis type
type Marshaller interface {
	Marshal() string
	GetValue() interface{}
}

// SimpleStrType defines a Simple String type in Redis RESP protocol
type SimpleStrType struct {
	Type
	Value string
}

// NewSimpleStr function constructs a new SimpleStrType object
func NewSimpleStr(value string) SimpleStrType {
	s := SimpleStrType{
		Type:  Type{prefix: SimplePrefix},
		Value: value,
	}
	return s
}

// Marshal implements the method of the Marshaller interface for SimpleStrType Class
func (s SimpleStrType) Marshal() string {
	return s.Type.prefix.toString() + s.Value + Separator
}

// GetValue implements the GetValue() method of the interface
func (s SimpleStrType) GetValue() interface{} {
	return s.Value
}

// ArrayType defines a Redis Array that could contain any other Redis Types
type ArrayType struct {
	Type
	Value []Marshaller
}

// NewArray creates a new array instance with the list of values of the Type generic type
func NewArray(values []Marshaller) ArrayType {
	return ArrayType{
		Type:  Type{prefix: ArrayPrefix},
		Value: values,
	}
}

// Marshal is the implementation of the Marshal() abstract function for the ArrayType type
func (a ArrayType) Marshal() string {
	numberElems := len(a.Value)
	if numberElems == 0 {
		return a.prefix.toString() + "0" + Separator
	}

	str := []string{a.prefix.toString(), strconv.Itoa(numberElems), Separator}

	for _, elem := range a.Value {
		str = append(str, elem.Marshal())
	}
	str = append(str, Separator)
	return strings.Join(str, "")
}

// GetValue implements the GetValue() method of the interface
func (a ArrayType) GetValue() interface{} {
	return a.Value
}

// ErrorType is the RESP representation of the Error Type
type ErrorType struct {
	Type
	Value string
}

// NewError is to be used to create a new ErrorType object
// TODO: Is it even needed
func NewError(value string) ErrorType {
	return ErrorType{
		Type:  Type{prefix: ErrorPrefix},
		Value: value,
	}
}

// Marshal is the implementation of the Marshal() method for the Error Type
// TODO: not sure if it is even needed...
func (e ErrorType) Marshal() string {
	return e.prefix.toString() + e.Value + Separator
}

// GetValue implements the GetValue() method of the interface
func (e ErrorType) GetValue() interface{} {
	return e.Value
}

// IntType represents Integer RESP object
type IntType struct {
	Type
	Value int
}

// NewInt function constructs a new Integer RESP object
func NewInt(value int) IntType {
	return IntType{
		Type:  Type{prefix: IntegerPrefix},
		Value: value,
	}
}

// Marshal in the implementation of the Marshal() method of the Marshaller interface{} for IntType
func (i IntType) Marshal() string {
	return i.prefix.toString() + strconv.Itoa(i.Value) + Separator
}

// GetValue implements the GetValue() method of the interface
func (i IntType) GetValue() interface{} {
	return i.Value
}

// BulkStrType is a Bulk string RESP representation
type BulkStrType struct {
	Type
	Value string
}

// NewBulkStr is a function to be called to construct a Bulk String object from the regular string
func NewBulkStr(value string) BulkStrType {
	return BulkStrType{
		Type:  Type{prefix: BulkStringPrefix},
		Value: value,
	}
}

// Marshal is the implementation of the Marshaller Marshal() method for the BulkStrType class
func (bs BulkStrType) Marshal() string {
	length := len(bs.Value)
	if length == 0 {
		return bs.prefix.toString() + "-1" + Separator
	}

	return bs.prefix.toString() + strconv.Itoa(length) + Separator + bs.Value + Separator
}

// GetValue implements the GetValue() method of the interface
func (bs BulkStrType) GetValue() interface{} {
	return bs.Value
}

// Command is a struct describing the Command that will be typically sent to the Redis server
// It is an array of bulk strings by definition
type Command struct {
	Type
	CommandName CommandStr
	Value       []Marshaller
}

// NewCommand function creates a new object for the command to be sent to the Redis server
func NewCommand(name CommandStr, value []Marshaller) Command {
	array := []Marshaller{NewBulkStr(name.toString())}
	array = append(array, value...)
	return Command{
		Type:        Type{prefix: ArrayPrefix},
		CommandName: name,
		Value:       array,
	}
}

// Marshal is the implementation of the Marshal() method of the Marshaller interface for the Command type
func (c Command) Marshal() string {
	// the lenCommand is the size of the array consisting of the len of Value []BulkStrType (command + arguments)
	lenArray := len(c.Value)

	str := []string{c.prefix.toString(), strconv.Itoa(lenArray), Separator}
	for _, elem := range c.Value {
		str = append(str, elem.Marshal())
	}

	return strings.Join(str, "")
}

// GetValue implements the GetValue() method of the interface
func (c Command) GetValue() interface{} {
	return c.Value
}

// Unmarshal is a function that takes a byte array as an input and tries to unmarhsal it to
// the supported RESP object(s)
func Unmarshal(data []byte) (Marshaller, error) {
	if data == nil || len(data) == 0 {
		return nil, errors.New("redis unmarshaller: the data byte array is nil or empty")
	}

	respType := string(data[0])

	if respType == ErrorPrefix.toString() {
		val := strings.Trim(string(data[1:]), Separator)
		return NewError(val), nil
	}

	if respType == SimplePrefix.toString() {
		val := strings.Trim(string(data[1:]), Separator)
		return NewSimpleStr(val), nil
	}

	if respType == IntegerPrefix.toString() {
		val := strings.Trim(string(data[1:]), Separator)
		valInt, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("redis unmarshaller: error converting the RESP int value (%v), the value itself cant be cast to int: %v", val, err)
		}
		return NewInt(valInt), nil
	}

	if respType == BulkStringPrefix.toString() {
		content := strings.Split(string(data[1:]), Separator)
		// the first item (index 0) is the lenght, second part is the content itself
		// if the lenght is -1 - that's a NULL string
		if content[0] == "-1" {
			return NewBulkStr(""), nil
		}
		return NewBulkStr(content[1]), nil
	}

	if respType == ArrayPrefix.toString() {
		strArray := strings.Trim(string(data[1:]), Separator)
		content := strings.Split(strArray, Separator)

		var value []Marshaller
		// the first element is number of elements, if 0 - that's an empty array
		length, err := strconv.Atoi(content[0])
		if err != nil {
			return nil, fmt.Errorf("redis unmarshaller: error converting array size %v into integer, possibly bad data in RESP %v", content[0], content)
		}
		if length == 0 {
			return NewArray(value), nil
		}

		for i := 1; i <= len(content[1:]); {

			// because the BulkString is weird and has separator in it after the length, there is a need
			// to combine the lenght content[x] and the value itself content[x+1]
			if strings.Index(content[i], BulkStringPrefix.toString()) != -1 {
				lenAndVal := content[i] + Separator + content[i+1] + Separator
				elem, err := Unmarshal([]byte(lenAndVal))
				if err != nil {
					return nil, err
				}
				value = append(value, elem)
				i += 2
				continue
			}

			elem, err := Unmarshal([]byte(content[i] + Separator))
			if err != nil {
				return nil, err
			}
			value = append(value, elem)
			i++
		}

		return NewArray(value), nil

	}

	return nil, fmt.Errorf("redis unmarshaller: error converting the byte array %v, no Marshaller implementation identified, possibly bad data",
		string(data[:]))
}

package gin_api_restriction

import (
	"bytes"
	"encoding/gob"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type RestrictionMiddleware interface {
	// returns whether the key is restricted, times remain, and error if there is any
	Validate(c *gin.Context, key string) (bool, *RestrictionEntity, error)
}

type RestrictionMiddlewareImpl struct {
	conf   *RestrictionConfig
	client *redis.Client
}

type RestrictionEntity struct {
	TotalLimit     int64
	TimesRemain    int64
	ResetTimeStamp int64
}

func (r *RestrictionMiddlewareImpl) Validate(c *gin.Context, key string) (bool, *RestrictionEntity, error) {
	res, err := r.client.Get(c, key).Result()
	switch {
	case err == redis.Nil:
		// create new k-v
		return true, r.createNewRecord(c, key), nil
	case err != nil:
		return false, nil, err
	default:
		// check timestamp
		success, entity := r.checkAndUpdateNewRecord(c, key, res)
		return success, entity, nil
	}
}

func (r *RestrictionMiddlewareImpl) createNewRecord(c *gin.Context, key string) *RestrictionEntity {
	record := &RestrictionEntity{
		TotalLimit:     r.conf.RestrictionCount,
		TimesRemain:    r.conf.RestrictionCount,
		ResetTimeStamp: time.Now().Unix() + int64(r.conf.RestrictionTime.Seconds()),
	}
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(record)
	if err != nil {
		panic(err)
	}
	err = r.client.Set(c, key, buf.String(), r.conf.RestrictionTime).Err()
	if err != nil {
		panic(err)
	}
	return record
}

func (r *RestrictionMiddlewareImpl) checkAndUpdateNewRecord(c *gin.Context, key string, recordBuf string) (bool, *RestrictionEntity) {
	record := &RestrictionEntity{}
	var buf bytes.Buffer
	decoder := gob.NewDecoder(&buf)
	buf.WriteString(recordBuf)
	err := decoder.Decode(record)
	if err != nil {
		panic(err)
	}
	if record.ResetTimeStamp <= time.Now().Unix() {
		// delete key and create new if already expired
		r.client.Del(c, key)
		record = r.createNewRecord(c, key)
		return true, record
	} else {
		if record.TimesRemain == 0 {
			return false, record
		} else {
			record.TimesRemain--
			return true, record
		}
	}
}

type RestrictionConfig struct {
	Log              bool
	RestrictionCount int64
	RestrictionTime  time.Duration
}

func NewRestrictionClient(conf *RestrictionConfig, rdbClient *redis.Client) RestrictionMiddleware {
	return &RestrictionMiddlewareImpl{
		client: rdbClient,
		conf:   conf,
	}
}

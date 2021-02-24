package gin_api_restriction

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
)

type RestrictionClient interface {
	// returns whether the key is restricted, times remain, and error if there is any
	Validate(c context.Context, key string) (bool, *RestrictionEntity, error)
}

type RestrictionClientImpl struct {
	conf   *RestrictionConfig
	client *redis.Client
}

type RestrictionEntity struct {
	TotalLimit     int64
	TimesRemain    int64
	ResetTimeStamp int64
}

func (r *RestrictionClientImpl) Validate(c context.Context, key string) (bool, *RestrictionEntity, error) {
	res, err := r.client.Get(c, key).Result()
	switch {
	case err == redis.Nil:
		// create new k-v
		record, err := r.createNewRecord(c, key)
		if err != nil {
			return false, nil, err
		}
		return true, record, nil
	case err != nil:
		return false, nil, err
	default:
		return r.checkAndUpdateNewRecord(c, key, res)
	}
}

func (r *RestrictionClientImpl) setRecord(c context.Context, key string, record *RestrictionEntity) error {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(record)
	if err != nil {
		return err
	}
	err = r.client.Set(c, key, buf.String(), r.conf.RestrictionTime).Err()
	return err
}

func (r *RestrictionClientImpl) createNewRecord(c context.Context, key string) (*RestrictionEntity, error) {
	record := &RestrictionEntity{
		TotalLimit:     r.conf.RestrictionCount,
		TimesRemain:    r.conf.RestrictionCount,
		ResetTimeStamp: time.Now().Unix() + int64(r.conf.RestrictionTime.Seconds()),
	}
	err := r.setRecord(c, key, record)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *RestrictionClientImpl) checkAndUpdateNewRecord(c context.Context, key string, recordBuf string) (bool, *RestrictionEntity, error) {
	record := &RestrictionEntity{}
	var buf bytes.Buffer
	decoder := gob.NewDecoder(&buf)
	buf.WriteString(recordBuf)
	err := decoder.Decode(record)
	if err != nil {
		return false, nil, errors.New("failed to decode the record")
	}
	if record.ResetTimeStamp <= time.Now().Unix() {
		// delete key and create new if already expired
		r.client.Del(c, key)
		record, err = r.createNewRecord(c, key)
		if err != nil {
			return false, nil, err
		}
		return true, record, nil
	} else {
		if record.TimesRemain == 0 {
			return false, record, nil
		} else {
			record.TimesRemain--
			err := r.setRecord(c, key, record)
			if err != nil {
				return false, nil, err
			}
			return true, record, nil
		}
	}
}

type RestrictionConfig struct {
	Log              bool
	RestrictionCount int64
	RestrictionTime  time.Duration
}

func NewRestrictionClient(conf *RestrictionConfig, rdbClient *redis.Client) RestrictionClient {
	return &RestrictionClientImpl{
		client: rdbClient,
		conf:   conf,
	}
}

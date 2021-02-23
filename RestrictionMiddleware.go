package gin_api_restriction

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type RestrictionMiddleWare struct {
	RestrictionClient
	// invoked when ip was rejected
	Reject func(ctx *gin.Context, entity *RestrictionEntity)
	// invoked when error occurs (most likely parsing and connection errors)
	Error func(ctx *gin.Context, err error)
	// invoked when everything is ok
	OK func(ctx *gin.Context, entity *RestrictionEntity)
}

func (r *RestrictionMiddleWare) RestrictionMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ok, entity, err := r.Validate(ctx, ctx.ClientIP())
		if err != nil {
			r.Error(ctx, err)
			return
		}
		ctx.Header("X-RateLimit-Limit", strconv.FormatInt(entity.TotalLimit, 10))
		ctx.Header("X-RateLimit-Remaining", strconv.FormatInt(entity.TimesRemain, 10))
		ctx.Header("X-RateLimit-Reset", strconv.FormatInt(entity.ResetTimeStamp, 10))
		if !ok {
			r.Reject(ctx, entity)
			return
		}
		r.OK(ctx, entity)
	}
}

func NewDefaultRestrictionMiddleWare(conf *RestrictionConfig, rdbClient *redis.Client) *RestrictionMiddleWare {
	return &RestrictionMiddleWare{
		RestrictionClient: NewRestrictionClient(conf, rdbClient),
		Reject:            defaultReject,
		Error:             defaultError,
		OK:                defaultOk,
	}
}

func defaultError(ctx *gin.Context, err error) {
	_ = ctx.AbortWithError(500, err).SetType(gin.ErrorTypePrivate)
}

func defaultReject(ctx *gin.Context, entity *RestrictionEntity) {
	_ = ctx.AbortWithError(403, errors.New("access limit exceeded, please check the headers and try again later")).SetType(gin.ErrorTypePublic)
}

func defaultOk(ctx *gin.Context, entity *RestrictionEntity) {
	ctx.Next()
}

package main

import (
	"context"
	"fmt"

	"github.com/lornshark/shark/sharkapp"
	"github.com/lornshark/shark/sharkdb"
	"github.com/lornshark/shark/sharkelastic"
	"github.com/lornshark/shark/sharkjson"
	"github.com/lornshark/shark/sharkkafka"
	"github.com/lornshark/shark/sharkmongodb"
	"github.com/lornshark/shark/sharkrabbitmq"
	"github.com/lornshark/shark/sharkredis"
	"github.com/lornshark/shark/sharkrisingwave"
	"github.com/spf13/cast"
)

type MyApp struct {
	*sharkapp.App
}

func main() {
	ctx := context.Background()
	configRedis, err := sharkredis.New(ctx, &sharkredis.Config{
		Host:     "192.168.191.100",
		Port:     6379,
		Password: "CEki57pxTJyYaLD",
	})
	if err != nil {
		panic(err)
	}
	project := "kgame"
	name := "game-test"
	id := "1"
	options := sharkapp.NewOption("dev", project, name, id)
	options.WithDb(sharkjson.ParseJsonString[sharkdb.Config](configRedis.Get(ctx, project+":system:config:db").Val()))
	options.WithRedis(sharkjson.ParseJsonString[sharkredis.Config](configRedis.Get(ctx, project+":system:config:redis").Val()))
	options.WithKafka(sharkjson.ParseJsonString[sharkkafka.Config](configRedis.Get(ctx, project+":system:config:kafka").Val()))
	options.WithElastic(sharkjson.ParseJsonString[sharkelastic.Config](configRedis.Get(ctx, project+":system:config:elastic").Val()))
	options.WithMongodb(sharkjson.ParseJsonString[sharkmongodb.Config](configRedis.Get(ctx, project+":system:config:mongodb").Val()))
	options.WithRabbitmq(sharkjson.ParseJsonString[sharkrabbitmq.Config](configRedis.Get(ctx, project+":system:config:rabbitmq").Val()))
	options.WithRisingwave(sharkjson.ParseJsonString[sharkrisingwave.Config](configRedis.Get(ctx, project+":system:config:risingwave").Val()))
	projectConfig := sharkjson.ParseJsonString[map[string]any](configRedis.Get(ctx, fmt.Sprintf("%v:system:config:%v-%v", project, name, id)).Val())
	options.WithTimer(cast.ToBool((*projectConfig)["timer"]))
	options.WithHttp(cast.ToInt((*projectConfig)["http"]))
	options.WithGrpc(cast.ToInt((*projectConfig)["grpc"]))
	options.WithPprof(cast.ToInt((*projectConfig)["pprof"]))
	options.WithHealth(cast.ToInt((*projectConfig)["health"]))

	app, err := sharkapp.New(options)
	if err != nil {
		panic(err)
	}
	app.Hunt()
}

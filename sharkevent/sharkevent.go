package sharkevent

import (
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/lornshark/shark/sharkrabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

// sharkevent 提供基于 RabbitMQ 的事件系统，支持事件发布和订阅，适用于微服务通信、消息通知、数据同步等场景。
// 不保证一定送达

type Event struct {
	env         string
	project     string
	name        string
	id          string
	rabbitmq    *sharkrabbitmq.Client
	handOffline bool // 是否处理离线消息
}

type eventmsg struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

func New(env, project, name, id string, rabbitmq *sharkrabbitmq.Client, handOffline bool) *Event {
	e := &Event{
		env:         env,
		project:     project,
		name:        name,
		id:          id,
		handOffline: handOffline,
		rabbitmq:    rabbitmq,
	}
	exchange := fmt.Sprintf("%v.event", project)
	queue := fmt.Sprintf("event.%v.%v.%v", project, name, id)
	if handOffline {
		rabbitmq.DeleteQueue(queue)
	}
	rabbitmq.Consume(exchange, queue, "event", e.consume)
	return e
}

func (e *Event) consume(msg *amqp.Delivery) {

}

func (e *Event) Publish(event string, data any) {
	ed, _ := sonic.Marshal(data)
	ev := eventmsg{
		Event: event,
		Data:  string(ed),
	}
	e.rabbitmq.Publish(fmt.Sprintf("%v.event", e.project), "event", ev)
}

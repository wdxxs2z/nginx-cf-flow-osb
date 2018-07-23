package mqworker

import (
	"github.com/pivotal-golang/lager"
	"github.com/wdxxs2z/nginx-flow-osb/config"
	"github.com/streadway/amqp"
	"fmt"
	"github.com/dmcgowan/msgpack"
	"errors"
)

type AMQPClient struct {
	connect		*amqp.Connection
	logger          lager.Logger
	queue           string
	exchange        string
}

func NewAMQPClient(config config.AMQPCred, logger lager.Logger)(*AMQPClient, error){
	qmqpUrl := fmt.Sprintf("amqp://%s:%s@%s:%d/", config.Username, config.Password, config.Host, config.Port)
	amqpConn, err := amqp.Dial(qmqpUrl)
	if err != nil {
		return nil, err
	}
	return &AMQPClient{
		connect:	amqpConn,
		logger:         logger,
		queue:          config.Queue,
		exchange:       config.Exchange,
	}
}

func (ac *AMQPClient)Send(msg interface{}) (error){
	ch, err := ac.connect.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	err = ch.ExchangeDeclare(ac.exchange, "topic", true, false, false, false, nil)
	if err != nil {
		return err
	}
	queueDeclare, err := ch.QueueDeclare(ac.queue, true, false, false, false, nil)
	if err != nil {
		return err
	}
	err = ch.QueueBind(ac.queue, ac.queue, ac.exchange, false, nil)
	if err != nil {
		return err
	}

	body, err := msgpack.Marshal(&msg)
	if err != nil {
		return err
	}
	publishing := amqp.Publishing{
		ContentType: "application/msgpack",
		Body:        body,
	}
	if err = ch.Publish(ac.exchange, queueDeclare.Name, false, false, publishing); err != nil {
		return err
	}
	return nil
}

func (ac *AMQPClient)Receive()(interface{}, error){
	ch, err := ac.connect.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	err = ch.ExchangeDeclare(ac.exchange, "topic", true, false, false, false, nil)
	if err != nil {
		return err
	}
	queueDeclare, err := ch.QueueDeclare(ac.queue, true, false, false, false, nil)
	if err != nil {
		return err
	}
	err = ch.QueueBind(ac.queue, ac.queue, ac.exchange, false, nil)
	if err != nil {
		return err
	}

	msg, ok, err := ch.Get(queueDeclare.Name, true)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Failed message")
	}
	msg.Ack(false)
	var data interface{}
	if err = msgpack.Unmarshal(msg, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (ac *AMQPClient)ReceiveChannel()(<- chan interface{}, error){
	ch := make(chan interface{})
	go func() {
		for {
			data, err := ac.Receive()
			if err != nil {
				continue
			}
			ch <- data
		}
	}()
	return ch, nil
}

func (ac *AMQPClient)SendChannel() (<- chan interface{}, error){
	ch := make(chan interface{})
	go func() {
		for {
			err := ac.Send(<- ch)
			if err != nil {
				continue
			}
		}
	}()
	return ch, nil
}
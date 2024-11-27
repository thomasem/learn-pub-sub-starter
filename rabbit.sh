#!/bin/bash

case "$1" in
    start)
        echo "Starting RabbitMQ container..."
        if docker inspect rabbitmq &>/dev/null; then
            docker start rabbitmq
        else
            docker run -d --name rabbitmq -p 5672:5672 -p 15672:15672 rabbitmq:3.13-management
        fi
        ;;
    stop)
        echo "Stopping RabbitMQ container..."
        docker stop rabbitmq
        ;;
    delete)
        echo "Deleting RabbitMQ container..."
        docker rm -fv rabbitmq
        ;;
    logs)
        echo "Fetching logs for RabbitMQ container..."
        docker logs -f rabbitmq
        ;;
    *)
        echo "Usage: $0 {start|stop|logs}"
        exit 1
esac

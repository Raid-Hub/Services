FROM rabbitmq:3.13-management

ENV RABBITMQ_PLUGINS_DIR=/opt/rabbitmq/plugins:/plugins

ADD https://github.com/rabbitmq/rabbitmq-delayed-message-exchange/releases/download/v3.13.0/rabbitmq_delayed_message_exchange-3.13.0.ez /plugins/rabbitmq_delayed_message_exchange-3.13.0.ez

RUN chmod 644 /plugins/rabbitmq_delayed_message_exchange-3.13.0.ez \
    && chown rabbitmq:rabbitmq /plugins/rabbitmq_delayed_message_exchange-3.13.0.ez \
    && rabbitmq-plugins enable --offline rabbitmq_delayed_message_exchange

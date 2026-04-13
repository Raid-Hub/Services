# Redis from Debian apt (same family as a bare-metal bullseye install), not the upstream redis Docker Hub image.
FROM debian:bullseye-slim

RUN apt-get update \
	&& apt-get install -y --no-install-recommends redis-server \
	&& rm -rf /var/lib/apt/lists/*

# Package defaults: loopback-only and systemd; fix for container networking and no systemd.
RUN sed -i 's/^bind 127.0.0.1 ::1/bind 0.0.0.0/' /etc/redis/redis.conf \
	&& sed -i 's/^bind 127.0.0.1$/bind 0.0.0.0/' /etc/redis/redis.conf \
	&& sed -i 's/^supervised systemd/supervised no/' /etc/redis/redis.conf \
	&& sed -i 's/^supervised auto/supervised no/' /etc/redis/redis.conf

# Keep docker-compose volume at redis_data:/data (matches old official redis image layout).
RUN mkdir -p /data \
	&& sed -i 's#^dir .*#dir /data#' /etc/redis/redis.conf \
	&& chown redis:redis /data

EXPOSE 6379

CMD ["redis-server", "/etc/redis/redis.conf", "--daemonize", "no"]

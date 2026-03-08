# syntax=docker/dockerfile:1.7

FROM nginxinc/nginx-unprivileged:1.27-alpine

COPY console/nginx.conf /etc/nginx/conf.d/default.conf
COPY console/index.html /usr/share/nginx/html/index.html
COPY console/runtime-config.template.js /opt/smith/runtime-config.template.js
COPY console/30-smith-env.sh /docker-entrypoint.d/30-smith-env.sh

EXPOSE 3000

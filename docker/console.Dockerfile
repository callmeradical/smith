# syntax=docker/dockerfile:1.7

# Build stage
FROM node:22-alpine as builder

WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm install --silent
COPY frontend/ ./
RUN npm run build

# Final stage
FROM nginxinc/nginx-unprivileged:1.27-alpine

COPY console/nginx.conf /etc/nginx/conf.d/default.conf
# Copy the built Svelte app from the builder stage
COPY --from=builder /src/frontend/build /usr/share/nginx/html
# Copy remaining assets if any (though SvelteKit build includes most)
COPY console/runtime-config.template.js /opt/smith/runtime-config.template.js
COPY console/30-smith-env.sh /docker-entrypoint.d/30-smith-env.sh

EXPOSE 3000

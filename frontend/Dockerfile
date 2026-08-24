FROM node:20-alpine AS build
WORKDIR /app
ARG NUXT_PUBLIC_API_BASE=http://localhost:8080
ARG NUXT_PUBLIC_WS_BASE=ws://localhost:8080
ENV NUXT_PUBLIC_API_BASE=$NUXT_PUBLIC_API_BASE
ENV NUXT_PUBLIC_WS_BASE=$NUXT_PUBLIC_WS_BASE
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run generate

FROM nginx:1.27-alpine AS producao
COPY --from=build /app/.output/public /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80

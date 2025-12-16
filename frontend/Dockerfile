# frontend/Dockerfile
FROM node:20-alpine
WORKDIR /app

COPY package.json package-lock.json* pnpm-lock.yaml* yarn.lock* ./

# Elige uno: npm (default)
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi

COPY . .
EXPOSE 5173

# Vite debe escuchar en 0.0.0.0 en contenedor
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0", "--port", "5173"]

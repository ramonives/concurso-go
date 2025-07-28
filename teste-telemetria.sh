#!/bin/bash

echo "🚀 Iniciando teste de telemetria com OpenTelemetry e Tempo"
echo ""

# Verificar se o docker-compose está rodando
echo "📋 Verificando se os serviços estão rodando..."
if ! docker ps | grep -q "concurso_tempo"; then
    echo "❌ Tempo não está rodando. Iniciando serviços..."
    cd docker
    docker-compose up -d
    cd ..
    echo "⏳ Aguardando serviços inicializarem..."
    sleep 10
else
    echo "✅ Serviços já estão rodando"
fi

echo ""
echo "🌐 URLs disponíveis:"
echo "   - Aplicação: http://localhost:8080"
echo "   - Grafana: http://localhost:3000"
echo "   - Tempo: http://localhost:3200"
echo ""

# Testar endpoints da aplicação
echo "🧪 Testando endpoints da aplicação..."

echo "1. Health check..."
curl -s http://localhost:8080/health | jq .

echo ""
echo "2. Populando dados..."
curl -s -X POST http://localhost:8080/start | jq .

echo ""
echo "3. Extraindo registros..."
curl -s -X POST http://localhost:8080/extrair/2024-01-15 | jq .

echo ""
echo "4. Consumindo registros..."
curl -s -X POST http://localhost:8080/consumir/2024-01-15 | jq .

echo ""
echo "5. Limpando Kafka..."
curl -s -X POST http://localhost:8080/limpar | jq .

echo ""
echo "✅ Teste concluído!"
echo ""
echo "📊 Para visualizar os traces:"
echo "   1. Acesse http://localhost:3000 (Grafana)"
echo "   2. Vá em 'Explore'"
echo "   3. Selecione 'Tempo' como datasource"
echo "   4. Digite: {service.name=\"concurso-go-app\"}"
echo "   5. Clique em 'Run Query'"
echo ""
echo "🔍 Para ver traces específicos no Tempo:"
echo "   - Acesse http://localhost:3200"
echo "   - Use a query: {service.name=\"concurso-go-app\"}" 
#!/bin/bash

echo "🚀 Testando Concurso Go App"
echo "=========================="

# Testar endpoint /start
echo "1. Criando tabelas e populando dados..."
curl -X POST http://localhost:8080/start
echo -e "\n"

# Aguardar um pouco
sleep 2

# Testar extração
echo "2. Extraindo registros do dia 15..."
curl -X POST http://localhost:8080/extrair/2025-01-15
echo -e "\n"

# Aguardar um pouco
sleep 2

# Testar consumo
echo "3. Consumindo registros do Kafka..."
curl -X POST http://localhost:8080/consumir/2025-01-15
echo -e "\n"

echo "✅ Teste concluído!" 
#!/bin/sh
# make init: prepara uma instalação nova para o setup pelo navegador. Nunca sobrescreve o que já existe.
#   1. cria o .env a partir do .env.example, se não existir;
#   2. cria a chave mestra em secrets/app_secret_key e aponta APP_SECRET_KEY_FILE para ela;
#   3. gera um SETUP_TOKEN aleatório (32 caracteres);
#   4. diz o que fazer em seguida.
set -eu
cd "$(dirname "$0")/.."

# value KEY: o valor de KEY no .env (vazio se não houver).
value() { sed -n "s/^$1=//p" .env | tail -n 1; }

# set_value KEY VALOR: grava KEY=VALOR no .env, trocando a linha vazia que existir (só é chamado com o valor vazio).
set_value() {
	tmp=$(mktemp)
	awk -v k="$1" -v v="$2" 'BEGIN { done = 0 } $0 ~ "^" k "=" && !done { print k "=" v; done = 1; next } { print } END { if (!done) print k "=" v }' .env > "$tmp"
	cat "$tmp" > .env
	rm -f "$tmp"
}

if [ ! -e .env ]; then
	cp .env.example .env
	chmod 600 .env
	echo "criado .env a partir do .env.example"
fi

key_file=/run/secrets/app_secret_key
if [ -n "$(value APP_SECRET_KEY_FILE)" ]; then
	echo "APP_SECRET_KEY_FILE já definida no .env: mantida"
elif [ -n "$(value APP_SECRET_KEY)" ]; then
	# Trocar a chave tornaria ilegíveis os segredos já salvos: quem usa a variável continua com ela.
	echo "APP_SECRET_KEY já definida no .env: mantida (para usar um arquivo, veja make secret-key no README)"
else
	if [ ! -e secrets/app_secret_key ]; then
		mkdir -p secrets
		(umask 077 && openssl rand -base64 32 > secrets/app_secret_key)
		echo "criado secrets/app_secret_key (chave mestra, fora do git)"
	fi
	set_value APP_SECRET_KEY_FILE "$key_file"
	echo "APP_SECRET_KEY_FILE=$key_file gravada no .env"
fi

if [ -n "$(value SETUP_TOKEN)" ]; then
	echo "SETUP_TOKEN já definido no .env: mantido"
else
	set_value SETUP_TOKEN "$(openssl rand -hex 16)"
	echo "SETUP_TOKEN gerado no .env"
fi

port=$(value WEB_PORT)
echo
echo "Pronto. Agora:"
echo "  1. docker compose up -d"
echo "  2. abra http://localhost:${port:-8080}"
echo "  3. cole o token do arquivo .env (linha SETUP_TOKEN)"

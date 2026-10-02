#!/bin/sh
# Backup do Postgres (pg_dump no formato custom) em /backups: o primeiro ao subir e depois a cada
# BACKUP_INTERVAL_HOURS (padrão 24), guardando só os BACKUP_KEEP mais recentes (padrão 14).
# Os arquivos têm todos os seus dados financeiros: ficam com permissão 600 e fora do git.
set -eu
umask 077

HOURS="${BACKUP_INTERVAL_HOURS:-24}"
KEEP="${BACKUP_KEEP:-14}"
case "$HOURS$KEEP" in
  '' | *[!0-9]*) echo "BACKUP_INTERVAL_HOURS e BACKUP_KEEP devem ser inteiros" >&2; exit 1 ;;
esac
if [ "$HOURS" -lt 1 ] || [ "$KEEP" -lt 1 ]; then
  echo "BACKUP_INTERVAL_HOURS e BACKUP_KEEP devem ser pelo menos 1" >&2
  exit 1
fi

while true; do
  stamp="$(date -u +%Y%m%d-%H%M%S)"
  tmp="/backups/.finassist-$stamp.tmp"
  out="/backups/finassist-$stamp.dump"
  # Grava num arquivo temporário e só renomeia se o pg_dump terminou: nunca fica um backup pela metade.
  if pg_dump -h postgres -U finassist -d finassist -Fc --no-owner -f "$tmp"; then
    mv "$tmp" "$out"
    echo "backup gravado: $out"
  else
    rm -f "$tmp"
    echo "ERRO: o backup falhou" >&2
  fi
  # Retenção: apaga os mais antigos além de BACKUP_KEEP.
  ls -1t /backups/finassist-*.dump 2>/dev/null | tail -n +"$((KEEP + 1))" | while read -r old; do rm -f -- "$old"; done
  sleep "$((HOURS * 3600))"
done

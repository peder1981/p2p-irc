#!/bin/sh

git filter-branch --env-filter '
# Email antigo da equipe
OLD_EMAIL1="detetive@exemplo.com"
OLD_EMAIL2="equipe@quemeaculpa.org"
OLD_NAME="Equipe de Quem é a Culpa"

# Informações corretas do autor
CORRECT_NAME="Peder Munksgaard"
CORRECT_EMAIL="peder@munksgaard.me"

# Verifica e corrige o committer
if [ "$GIT_COMMITTER_EMAIL" = "$OLD_EMAIL1" ] || [ "$GIT_COMMITTER_EMAIL" = "$OLD_EMAIL2" ] || [ "$GIT_COMMITTER_NAME" = "$OLD_NAME" ]
then
    export GIT_COMMITTER_NAME="$CORRECT_NAME"
    export GIT_COMMITTER_EMAIL="$CORRECT_EMAIL"
fi

# Verifica e corrige o autor
if [ "$GIT_AUTHOR_EMAIL" = "$OLD_EMAIL1" ] || [ "$GIT_AUTHOR_EMAIL" = "$OLD_EMAIL2" ] || [ "$GIT_AUTHOR_NAME" = "$OLD_NAME" ]
then
    export GIT_AUTHOR_NAME="$CORRECT_NAME"
    export GIT_AUTHOR_EMAIL="$CORRECT_EMAIL"
fi
' --tag-name-filter cat -- --branches --tags

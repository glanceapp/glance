# Configurer et utiliser Glance pour AfriCompta

## IMPORTANT: Deux versions disponibles

Deux applications sont disponibles dans ce répertoire :

1. **Application Glance officielle** (recommandée) - via Docker
2. **Version personnalisée Node.js** - pour développement uniquement

## Option 1: Utiliser Glance officiel (recommandé)

1. Assurez-vous que Docker est installé et en cours d'exécution
2. Exécutez la commande suivante pour démarrer l'application:

```bash
docker compose up -d
```

3. Accédez à l'application via votre navigateur: http://localhost:8080

4. Pour arrêter l'application:

```bash
docker compose down
```

## Option 2: Utiliser la version Node.js (développement)

Cette version est utile uniquement pour le développement local.

1. Assurez-vous que Node.js est installé
2. Exécutez:

```bash
npm start
```

3. Accédez à l'application via votre navigateur: http://localhost:3001

## Structure des fichiers

- `config/glance.yml` - Configuration principale de Glance
- `config/accounting-dashboard.yml` - Configuration du tableau de bord comptable
- `docker-compose.yml` - Configuration Docker
- `index.js` - Application Node.js alternative

## Dépannage

Si vous rencontrez des problèmes avec la version Docker:

```bash
# Vérifier les logs
docker logs glance

# Redémarrer le conteneur
docker restart glance

# Reconstruire le conteneur
docker compose down
docker compose up -d
```

Si le conteneur ne démarre pas correctement:

```bash
# Vérifier l'état de tous les conteneurs (même ceux arrêtés)
docker ps -a

# Vérifier les logs même si le conteneur s'est arrêté
docker logs glance

# Vérifier les permissions des dossiers
ls -la config/

# S'assurer que le fichier de configuration est accessible
cat config/glance.yml

# Redémarrer avec plus de verbosité
docker compose down
docker compose up
```

### Vérifications supplémentaires

1. Assurez-vous que le répertoire config et ses fichiers sont accessibles:
   ```bash
   chmod -R 755 config/
   ```

2. Si le problème persiste, essayez d'exécuter le conteneur en mode interactif:
   ```bash
   docker run --rm -it -v $(pwd)/config:/app/config -p 8080:8080 glanceapp/glance
   ```

## ✅ Installation réussie !

Votre tableau de bord AfriCompta est maintenant opérationnel et accessible à l'adresse:
http://localhost:8080

### Utilisation quotidienne

1. **Démarrer le tableau de bord**:
   ```bash
   docker compose up -d
   ```

2. **Arrêter le tableau de bord**:
   ```bash
   docker compose down
   ```

3. **Vérifier l'état du conteneur**:
   ```bash
   docker ps
   ```

4. **Visualiser les logs**:
   ```bash
   docker logs glance
   ```

### Personnalisation du contenu

Pour modifier les données affichées:

1. Éditez le fichier `config/accounting-dashboard.yml` pour changer les valeurs, widgets, ou structure.
2. Redémarrez le conteneur pour appliquer vos modifications:
   ```bash
   docker compose restart
   ```

### Sauvegardes

Pour sauvegarder votre configuration:

```bash
cp -r config/ config-backup-$(date +%Y%m%d)/
```

## Ressources supplémentaires

- Documentation officielle: https://github.com/glanceapp/glance/tree/main/docs
- Thèmes disponibles: https://github.com/glanceapp/glance/blob/main/docs/themes.md
- Pages préconfigurées: https://github.com/glanceapp/glance/blob/main/docs/preconfigured-pages.md

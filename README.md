<p align="center"><em>Simplifiez votre comptabilité avec...</em></p>
<h1 align="center">AfriCompta</h1>
<p align="center"><a href="#installation">Installation</a> • <a href="docs/configuration.md">Configuration</a> • <a href="docs/modules.md">Modules</a> • <a href="docs/themes.md">Thèmes</a></p>

![](docs/images/readme-main-image.png)

## Fonctionnalités
### Modules comptables variés
* Tableaux de bord financiers
* Suivi des revenus et dépenses
* Rapports fiscaux adaptés aux pays africains
* Gestion des factures et devis
* Conversion automatique des devises africaines
* Suivi des taxes (TVA, IS, BIC) 
* Intégration mobile money (MTN, Orange, Wave)
* Gestion des immobilisations
* Rapports personnalisables
* [et bien plus...](docs/configuration.md)

### Rapide et léger
* Faible consommation de mémoire
* Peu de dépendances
* JavaScript minimaliste
* Application disponible pour plusieurs systèmes d'exploitation
* Chargement rapide même avec une connexion internet limitée

### Hautement personnalisable
* Différentes mises en page
* Autant de modules que nécessaire
* Nombreuses options de configuration pour chaque module
* Plusieurs styles disponibles
* CSS personnalisable

### Optimisé pour les appareils mobiles
Parce que la comptabilité doit être accessible partout en Afrique.

![](docs/images/mobile-preview.png)

### Personnalisation des thèmes
Créez facilement votre propre thème ou choisissez parmi ceux déjà disponibles.

![](docs/images/themes-example.png)

<br>

## Configuration
La configuration s'effectue via des fichiers YAML. Pour en savoir plus sur le fonctionnement de la mise en page, l'ajout de modules et leur configuration, consultez la [documentation de configuration](docs/configuration.md).

<details>
<summary><strong>Aperçu d'un fichier de configuration</strong></summary>
<br>

```yaml
pages:
  - name: Tableau de Bord
    columns:
      - size: small
        widgets:
          - type: calendar
            first-day-of-week: monday

          - type: income-summary
            period: monthly
            currency: XOF
            collapse-after: 3
            cache: 12h
            sources:
              - name: Commerce
                color: "#4287f5"
              - name: Services
                color: "#42f5a7"
              - name: Investissements
                color: "#f5a742"

          - type: expense-tracking
            categories:
              - loyer
              - personnel
              - matériel
              - transport
              - marketing
              - taxes

      - size: full
        widgets:
          - type: group
            widgets:
              - type: financial-summary
              - type: tax-calculator

          - type: invoice-tracker
            status:
              - payée
              - en attente
              - retard
              - annulée

          - type: group
            widgets:
              - type: currency-exchange
                base: XOF
                target:
                  - EUR
                  - USD
                  - NGN
                  - GHS
              - type: market-trends
                markets:
                  - BRVM
                  - NSE
                  - JSE

      - size: small
        widgets:
          - type: weather
            location: Dakar, Sénégal
            units: metric
            hour-format: 24h

          - type: financial-alerts
            alerts:
              - type: tax-deadline
                name: TVA mensuelle
                date: 15
              - type: payment-due
                name: Salaires
                date: 30
              - type: reconciliation
                name: Rapprochement bancaire
                date: 5

          - type: regulatory-updates
            regions:
              - UEMOA
              - CEMAC
              - national
```
</details>

<br>

## Installation

Choisissez l'une des méthodes suivantes:

<details>
<summary><strong>Docker compose (recommandé)</strong></summary>
<br>

Pour installer AfriCompta, vous devez créer la structure de fichiers manuellement:

```bash
# Créer la structure de répertoires
mkdir -p africompta/{config,assets}
cd africompta

# Créer le fichier docker-compose.yml
cat > docker-compose.yml << 'EOF'
services:
  app:
    container_name: africompta
    image: africompta/app:latest
    volumes:
      - ./config:/app/config
      - ./assets:/app/assets
    ports:
      - 8080:8080
    restart: unless-stopped
EOF

# Créer le fichier de configuration principal
cat > config/config.yml << 'EOF'
title: AfriCompta
theme: default
language: fr
pages:
  - include: accounting-dashboard.yml
EOF

# Créer le fichier du tableau de bord comptable
cat > config/accounting-dashboard.yml << 'EOF'
name: Tableau de Bord
columns:
  - size: full
    widgets:
      - type: html
        content: |
          <div style="text-align: center">
            <h1>Bienvenue sur AfriCompta</h1>
            <p>Votre solution de comptabilité adaptée au contexte africain</p>
          </div>
  - size: small
    widgets:
      - type: calendar
        first-day-of-week: monday
      - type: html
        title: À faire
        content: |
          <ul>
            <li>Configurer mes comptes</li>
            <li>Ajouter mes transactions</li>
            <li>Générer mes premiers rapports</li>
          </ul>
  - size: small
    widgets:
      - type: html
        title: Statistiques
        content: |
          <div style="padding: 15px">
            <p><strong>Factures en attente:</strong> 0</p>
            <p><strong>Paiements ce mois:</strong> 0 XOF</p>
            <p><strong>Dépenses ce mois:</strong> 0 XOF</p>
          </div>
EOF

# Créer un fichier CSS personnalisé
mkdir -p assets
cat > assets/user.css << 'EOF'
/* Votre CSS personnalisé ici */
.dashboard-title {
  color: #2c3e50;
}
EOF

echo "Structure AfriCompta créée avec succès!"
```

Après avoir exécuté ce script, vous pourrez démarrer AfriCompta avec:

```bash
docker compose up -d
```

En cas de problème, vous pouvez consulter les logs:

```bash
docker compose logs
```

<hr>
</details>

<details>
<summary><strong>Installation manuelle</strong></summary>
<br>

Créez un fichier `docker-compose.yml` avec le contenu suivant:

```yaml
services:
  app:
    container_name: africompta
    image: africompta/app
    volumes:
      - ./config:/app/config
    ports:
      - 8080:8080
```

Ensuite, créez un nouveau répertoire appelé `config` et téléchargez le fichier de configuration de base.

Modifiez le fichier de configuration selon vos besoins, puis exécutez:

```bash
docker compose up -d
```

En cas de problème, consultez les logs:

```bash
docker logs africompta
```

<hr>
</details>

<br>

# Configuration et utilisation d'AfriCompta

## IMPORTANT: Deux versions disponibles

Deux applications sont disponibles dans ce dépôt:

1. **Application officielle** (recommandée) - via Docker
2. **Version Node.js personnalisée** - pour le développement uniquement

## Option 1: Utiliser l'application officielle (recommandée)

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

- `config/config.yml` - Configuration principale
- `config/accounting-dashboard.yml` - Configuration du tableau de bord comptable
- `docker-compose.yml` - Configuration Docker
- `index.js` - Application Node.js alternative

## Dépannage

Si vous rencontrez des problèmes avec la version Docker:

```bash
# Vérifier les logs
docker logs africompta

# Redémarrer le conteneur
docker restart africompta

# Reconstruire le conteneur
docker compose down
docker compose up -d
```

Si le conteneur ne démarre pas correctement:

```bash
# Vérifier l'état de tous les conteneurs
docker ps -a

# Vérifier les logs même si le conteneur est arrêté
docker logs africompta

# Vérifier les permissions des dossiers
ls -la config/

# S'assurer que le fichier de configuration est accessible
cat config/config.yml

# Redémarrer avec plus de détails
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
   docker run --rm -it -v $(pwd)/config:/app/config -p 8080:8080 africompta/app
   ```

## ✅ Installation réussie!

Votre tableau de bord AfriCompta est maintenant opérationnel et accessible à:
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

4. **Consulter les logs**:
   ```bash
   docker logs africompta
   ```

### Personnalisation du contenu

Pour modifier les données affichées:

1. Modifiez le fichier `config/accounting-dashboard.yml` pour changer les valeurs, widgets ou la structure.
2. Redémarrez le conteneur pour appliquer vos changements:
   ```bash
   docker compose restart
   ```

### Sauvegardes

Pour sauvegarder votre configuration:

```bash
cp -r config/ config-backup-$(date +%Y%m%d)/
```

## Ressources additionnelles

- Documentation officielle: Consultez le répertoire docs
- Thèmes disponibles: Consultez la documentation des thèmes
- Modules préconfigurés: Consultez la documentation des modules préconfigurés

<br>

## Problèmes courants
<details>
<summary><strong>Dépassement du délai de requête</strong></summary>

En cas de connexion internet limitée ou instable, commune dans certaines régions africaines, vous pouvez rencontrer des problèmes de timeout. Solutions:

1. Configurez des temps d'attente plus longs dans le fichier config.yml:
```yaml
request-timeout: 30s
```

2. Activez la mise en cache locale pour les modules critiques:
```yaml
cache-strategy: aggressive
cache-duration: 24h
```
</details>

<details>
<summary><strong>Problèmes de devises</strong></summary>

Si les taux de change ne s'affichent pas correctement, vérifiez que vous avez configuré votre source de taux de change et les devises correctement:

```yaml
currency-settings:
  base: XOF
  api-key: votre-clé-api
  preferred-sources: [bceao, afdb, custom]
```
</details>

<details>
<summary><strong>Problèmes de codes fiscaux</strong></summary>

Si les calculs fiscaux ne correspondent pas à votre pays, assurez-vous d'avoir sélectionné le bon régime fiscal:

```yaml
tax-system:
  country: senegal  # options: senegal, cameroun, cote-ivoire, etc.
  regime: standard  # options: standard, simplifié, etc.
```
</details>

<br>

## FAQ
<details>
<summary><strong>Les données comptables se mettent-elles à jour automatiquement?</strong></summary>
Non, un rafraîchissement de la page est nécessaire pour mettre à jour les informations. Certains éléments se mettent à jour dynamiquement lorsque cela est pertinent, comme l'horloge et les délais.
</details>

<details>
<summary><strong>À quelle fréquence les modules se mettent-ils à jour?</strong></summary>
Les informations sont récupérées uniquement lors du chargement de la page, puis mises en cache. La durée de vie du cache est différente pour chaque module et peut être configurée.
</details>

<details>
<summary><strong>Puis-je créer mes propres modules comptables?</strong></summary>

Oui, il existe plusieurs façons de créer des modules personnalisés:
* Module `iframe` - permet d'intégrer des éléments d'autres sites
* Module `html` - permet d'insérer votre propre HTML statique
* Module `extension` - récupère du HTML depuis une URL
* Module `custom-api` - récupère du JSON depuis une URL et le rend avec du HTML personnalisé
</details>

<details>
<summary><strong>L'application est-elle conforme aux normes comptables africaines?</strong></summary>
Oui, AfriCompta prend en charge les normes SYSCOHADA, SYSCOA et les réglementations fiscales spécifiques à de nombreux pays africains. Vous pouvez sélectionner votre pays dans les paramètres pour adapter automatiquement les calculs et les rapports.
</details>

<br>

## Compilation depuis les sources

Choisissez l'une des méthodes suivantes:

<details>
<summary><strong>Compilation avec Go</strong></summary>
<br>

Prérequis: [Go](https://go.dev/dl/) >= v1.23

Pour compiler le projet pour votre OS et architecture actuels, exécutez:

```bash
go build -o build/app .
```

Pour compiler pour un OS et une architecture spécifiques:

```bash
GOOS=linux GOARCH=amd64 go build -o build/app .
```

[*cliquez ici pour une liste complète des combinaisons GOOS et GOARCH*](https://go.dev/doc/install/source#:~:text=$GOOS%20and%20$GOARCH)

Alternativement, pour exécuter l'application sans créer de binaire:

```bash
go run .
```
<hr>
</details>

<details>
<summary><strong>Compilation avec Docker</strong></summary>
<br>

Prérequis: [Docker](https://docs.docker.com/engine/install/)

Pour compiler le projet et l'image en utilisant Docker:

*(remplacez `owner` par votre nom ou organisation)*

```bash
docker build -t owner/africompta:latest .
```

Pour pousser l'image vers un registre:

```bash
docker push owner/africompta:latest
```

<hr>
</details>

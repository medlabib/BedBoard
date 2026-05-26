# BedBoard

<p align="center">
  <img src="logo.svg" alt="BedBoard" width="120" />
</p>

<p align="center">
  <img alt="Déploiement Local" src="https://img.shields.io/badge/D%C3%A9ploiement%20Local-Rapide%20et%20S%C3%A9curis%C3%A9-B5C7A4" />
  <img alt="Temps Réel" src="https://img.shields.io/badge/Temps%20R%C3%A9el-Visibilit%C3%A9%20des%20lits-A6B8C7" />
  <img alt="Parcours Patient" src="https://img.shields.io/badge/Parcours%20Patient-De%20l'entr%C3%A9e%20%C3%A0%20l'archive-D4B4A5" />
  <img alt="Accès" src="https://img.shields.io/badge/Acc%C3%A8s-S%C3%A9curis%C3%A9-E2DACD" />
</p>

<p align="center">
  <code style="background:#B5C7A4;color:#1e1a17;padding:4px 8px;border-radius:999px;">#B5C7A4</code>
  <code style="background:#A6B8C7;color:#1e1a17;padding:4px 8px;border-radius:999px;">#A6B8C7</code>
  <code style="background:#D4B4A5;color:#1e1a17;padding:4px 8px;border-radius:999px;">#D4B4A5</code>
  <code style="background:#E2DACD;color:#1e1a17;padding:4px 8px;border-radius:999px;">#E2DACD</code>
</p>

**BedBoard** permet aux équipes médicales et paramédicales de répondre à la question opérationnelle la plus cruciale d'un service en un coup d'œil : 

**Quels lits sont actuellement disponibles, quel patient y est installé, et quelle est la prochaine étape de sa prise en charge ?**

## Le Défi du Quotidien

Dans un environnement sous tension, les équipes soignantes perdent un temps précieux à cause d'une information fragmentée :
* Les transmissions orales ou les notes manuscrites créent un décalage entre le tableau de bord et la réalité clinique.
* Les changements de statut des lits (libération, bionettoyage en cours) ne sont pas visibles simultanément par le triage, les médecins et les infirmiers.
* Lors des pics d'affluence, le suivi des étapes (attente de consultation, décision de sortie) manque de fluidité.
* Les cadres de santé manquent d'une vision globale et instantanée de l'occupation du service.

## L'Approche BedBoard

Conçu spécifiquement pour un fonctionnement intra-hospitalier, BedBoard privilégie la vitesse de saisie et la clarté visuelle.

* **Cartographie en direct :** Un tableau de bord unique reflétant l'occupation réelle du service.
* **Gestion des flux :** Cycle de vie clair du patient (en attente, installé, consulté, archivé/sortant).
* **Mode Affichage :** Une vue plein écran optimisée pour les moniteurs des postes de soins.
* **Séparation des rôles :** Les soignants gèrent les patients et les lits ; les administrateurs gèrent la configuration de l'unité et les accès.
* **Fiabilité locale :** Une architecture "local-first" qui garantit une réactivité maximale sans dépendre d'une connexion cloud externe.

## Bénéfices pour les Équipes

* **Fluidité des transmissions :** Des prises de décision accélérées lors des changements de garde.
* **Réduction des erreurs :** Une meilleure coordination entre la zone de triage et la zone de soins.
* **Anticipation :** Une visualisation immédiate de la tension et de la disponibilité hospitalière.

## Parcours Type sur l'Application

1. **Identification :** Ajout rapide du patient sur le tableau de bord (création d'une entrée avec statut *en attente* ou *assigné*).
2. **Installation :** Attribution d'un lit au patient directement depuis la cartographie centrale ou la liste d'attente.
3. **Suivi du Lit :** Mise à jour en un clic du statut de l'emplacement pendant les soins (occupé, en nettoyage, alerte médicale, libre).
4. **Validation :** Marquage de la consultation comme effectuée une fois le patient vu par le médecin.
5. **Sortie :** Archivage de l'entrée du patient et libération immédiate du lit pour fluidifier l'aval.
6. **Analyse :** Consultation de la vue statistique pour refléter l'activité journalière du service.

---

## Nouveautés de la v2.1

La dernière mise à jour se concentre sur la sécurité patient et la traçabilité des actions :

* **Prévention des conflits (Anti-collision) :** Un système de verrouillage logique empêche d'attribuer accidentellement un même lit à deux patients simultanément.
* **Traçabilité totale :** Un journal d'audit complet enregistre chaque mouvement (qui a modifié l'état d'un lit, à quelle heure, et le statut précédent/actuel).
* **Protection des accès renforcée :** Politique de mots de passe hautement paramétrable (longueur, caractères exigés, renouvellement automatique, verrouillage après tentatives échouées).
* **Sauvegarde simplifiée :** L'administration permet désormais la sauvegarde et la restauration de la base de données en un seul clic.
* **Performance et ergonomie :** Interface optimisée avec mise en cache locale pour des transitions sans clignotement, et actualisation instantanée par événements (fini le rafraîchissement manuel).

---

## Guide Technique et Déploiement

BedBoard est conçu pour être déployé sur le réseau privé du centre de soins. En cas d'ouverture sur l'extérieur, il est impératif de le placer derrière un protocole HTTPS avec des politiques strictes de restriction d'accès réseau.

### Démarrage Rapide

Pour compiler et lancer le serveur en local :

```bash
npm --prefix frontend ci
npm --prefix frontend run build
go run .
# Cahier des charges MES — Usine de sous-traitance aéronautique
## Contexte : remplacement de Shopvue

---

## 1. Gestion des Ordres de Fabrication (OF)

- Création d'OF manuellement ou par import depuis l'ERP (CSV, API)
- Champs obligatoires : référence pièce, numéro de série / numéro de lot, gamme associée et sa version, quantité, date de besoin, priorité
- Statuts complets et tracés : Planifié → Lancé → En cours → Suspendu → Terminé → Clôturé
- Historique complet des changements de statut avec horodatage et opérateur responsable
- Possibilité de suspendre un OF en cours avec motif obligatoire (attente matière, panne machine, décision qualité)
- Division d'un OF en sous-lots (split) et regroupement de lots (merge) avec réaffectation automatique des quantités
- Rebut partiel : clôture d'un OF avec quantité produite inférieure à la quantité planifiée, motif obligatoire
- Priorisation dynamique : modification de l'ordre de passage des OF en file d'attente par le superviseur ou le responsable de production
- Gestion multi-niveaux : assemblages, sous-ensembles, pièces unitaires avec liens parent/enfant
- Visualisation du plan de charge par machine / poste sur un horizon glissant (Gantt simplifié ou board)
- Recherche rapide d'un OF par référence, numéro de lot, numéro de série, date, statut

---

## 2. Routage et gammes opératoires

- Association d'une gamme versionnée à chaque OF — version applicable figée au lancement, non modifiable sans autorisation
- Gamme composée d'opérations ordonnées avec : numéro d'opération, intitulé, poste de travail cible, temps alloué (setup + cycle), instructions de travail associées
- Gestion des opérations conditionnelles (opération déclenchée selon le résultat d'un contrôle précédent)
- Gestion des opérations parallèles (deux opérations pouvant se dérouler simultanément)
- Gestion des retouches et gammes de réparation : insertion d'opérations de retouche non prévues dans la gamme initiale, ou bascule sur une gamme de réparation spécifique avec validation hiérarchique obligatoire
- Blocage automatique du passage à l'opération suivante si l'opération en cours n'est pas déclarée terminée
- Gestion des dérogations et waivers : workflow d'approbation traçable avant toute déviation à la gamme standard
- Affichage gamme en lecture seule pour les opérateurs — toute modification en cours de production nécessite une autorisation explicite et tracée

---

## 3. Interface opérateur en bord de ligne

- Interface tactile optimisée pour tablette industrielle 10 à 12 pouces, utilisable avec des gants de travail
- Composants larges, contrastes élevés, temps de réponse < 2 secondes sur toute interaction
- Connexion rapide par badge RFID, QR code ou code PIN court — pas de saisie de mot de passe complexe en bord de ligne
- Vue opérateur : uniquement les OF assignés à son poste, dans l'ordre de priorité
- Affichage clair de l'opération en cours : intitulé, instructions de travail électroniques (EWI), photos, plans 2D, tolérances, outillages requis
- Navigation pas à pas entre opérations : l'opérateur ne voit que l'opération courante
- Collecte de données contextuelles bloquante : saisie obligatoire des paramètres de process (couples de serrage, températures, pressions) — l'OF ne peut pas avancer si la valeur est manquante ou hors tolérance
- Chronomètre intégré : démarrage / arrêt / pause du temps de cycle directement depuis l'écran
- Déclaration des quantités produites (bonnes + rebuts) par opération
- Signalement d'un aléa en 2 actions maximum : panne machine, manque matière, problème qualité, accident — motif sélectionnable dans une liste préétablie
- Journal de bord et relève de quart : outil de communication intégré pour transmettre consignes, problèmes en cours et état des encours entre équipes (ex. 3x8)
- Mode dégradé offline : fonctionnement sans connexion réseau, synchronisation automatique au rétablissement

---

## 4. Traçabilité lot et numéro de série

- Chaque pièce produite est obligatoirement associée à : son OF, sa gamme et sa version, chaque opération réalisée, l'opérateur par opération, le poste de travail par opération, les lots matière consommés avec certificats associés, les outillages et moyens de mesure utilisés, la date et heure de chaque événement
- Traçabilité ascendante : depuis une pièce finie, reconstituer toute la chaîne de production
- Traçabilité descendante : depuis un lot matière, identifier toutes les pièces produites avec ce lot
- As-Built Record (Dossier Industriel Numérique) : constitution automatique du "tel que construit" — ce qui a été monté, par qui, sur quelle machine, avec quels outils, à quelle heure
- Généalogie complète reconstructible en moins de 10 secondes depuis l'interface ou l'API
- Gestion du sériel : numéro de série unique par pièce avec vérification de doublon à la saisie
- Gestion de la péremption matières : blocage automatique si date de péremption ou Time Out of Environment (TOE) dépassé — applicable aux mastics, résines, composites, colles
- Historique de traçabilité conservé sans limite de durée, non altérable (audit trail immuable)
- Gestion des FAI (First Article Inspection) : flag spécifique et workflow d'approbation dédié avec points d'arrêt obligatoires pour la validation qualité avant toute production série

---

## 5. Suivi des temps et calcul TRS / OEE

- Saisie des temps par opération : temps de setup (changement de série), temps de cycle effectif, temps d'arrêt avec motif catégorisé
- Catégories d'arrêt obligatoires : panne machine, maintenance préventive, manque matière, attente qualité, changement de série, pause réglementaire, arrêt non justifié
- Calcul automatique du TRS en temps réel sur les OF actifs : Disponibilité × Performance × Qualité
- Dashboard TRS accessible sans calcul manuel, mis à jour en continu
- Historique du TRS par machine, par ligne, par référence, par période, par équipe
- Comparaison TRS réel vs TRS cible paramétrable par poste
- Identification des top causes de perte de TRS sur une période donnée
- Système Andon : déclenchement d'appels à l'aide (maintenance, qualité, logistique) depuis le poste de travail avec escalade visuelle dans l'atelier
- Export des données TRS brutes via API pour intégration dans des outils de reporting tiers

---

## 6. Gestion des ressources et postes de travail

- Référentiel des machines et postes de travail : identifiant, désignation, capacité théorique, cadence nominale, statut courant (disponible / en production / en arrêt / en maintenance)
- Connectivité machines native : remontée automatique des compteurs de production, temps de cycle et paramètres via OPC-UA, MTConnect ou MQTT — sans saisie manuelle opérateur
- Affectation d'un OF à une machine spécifique ou à une famille de machines équivalentes
- Statut machine en temps réel depuis le dashboard superviseur
- Alertes automatiques si une machine est déclarée en arrêt depuis plus de X minutes (seuil paramétrable)
- Gestion des équipes et des shifts : association des OF et des déclarations à une équipe, un shift et un superviseur
- Intégration périphériques atelier : douchettes code-barres / QR codes, lecteurs RFID (bacs, outillages), imprimantes d'étiquettes industrielles

---

## 7. Gestion des opérateurs et habilitations

- Référentiel opérateurs : nom, identifiant, poste(s) autorisé(s), habilitations avec date d'expiration (procédés spéciaux, CND, soudure, ressuage, postes critiques)
- Verrouillage au poste (Interlock) : impossibilité pour un opérateur de démarrer une opération s'il n'a pas les habilitations à jour requises pour cette tâche précise
- Alertes automatiques à l'approche de l'échéance d'une habilitation (délai paramétrable)
- Blocage automatique d'un opérateur dont l'habilitation est expirée sur les opérations concernées
- Pointage opérateur (Clock-in / Clock-out) sur les OF pour calcul du coût de revient réel de la pièce
- Historique complet de toutes les actions réalisées par opérateur — traçabilité individuelle non altérable
- Gestion des absences et remplacements : réaffectation des OF en cours en cas d'absence imprévue

---

## 8. Gestion des outillages et moyens de mesure

- Référentiel des outillages et équipements de contrôle avec statut de validité de calibration / étalonnage
- Vérification systématique avant le début d'une opération : outil hors date d'étalonnage = blocage de l'opération
- Suivi de la durée de vie et des cycles d'utilisation des outils de coupe et consommables machines, avec alertes de remplacement
- Association des outillages utilisés à chaque opération de l'OF (traçabilité outil par pièce)
- Alerte automatique si un outil dont la calibration a expiré est scanné en bord de ligne

---

## 9. Suivi des matières et composants consommés

- Déclaration des matières consommées par OF : référence matière, numéro de lot, quantité consommée
- Vérification que le lot matière déclaré est conforme : non bloqué, certificat valide, péremption non dépassée, TOE respecté
- Alerte et blocage si un lot matière avec statut bloqué ou périmé est scanné en bord de ligne
- Calcul des écarts matière : consommé réel vs consommé théorique (basé sur la gamme)
- Suivi des encours (WIP) : localisation des pièces en cours dans le flux atelier
- Gestion des transferts inter-postes : déclaration des mouvements entre postes de travail avec horodatage et opérateur

---

## 10. Qualité inline (contrôles en cours de production)

- Saisie des contrôles qualité depuis l'interface opérateur : valeurs métrologiques, cotes dimensionnelles, paramètres process
- Validation tolérance instantanée à la saisie — alerte visuelle immédiate si hors tolérance
- Règles SPC (Statistical Process Control) intégrées : alertes sur les dérives de process avant déclaration de non-conformité
- Déclaration d'une non-conformité depuis l'écran OF en 2 actions maximum : isolation informatique immédiate du lot ou de la pièce concernée
- Statuts NC : Ouverte → En analyse → En traitement → Clôturée
- Workflow de traitement des NC : retouche, dérogation client, rebut — chaque chemin est tracé et validé
- Blocage automatique de toute opération aval sur un lot ou une pièce déclarée non conforme
- Enregistrement des résultats de tests spéciaux : CND, ressuage, radiographie — avec association à l'OF et à la pièce

---

## 11. Gestion des événements et alertes

- Tableau de bord des aléas en cours, visible par le superviseur et le responsable de production
- Alertes configurables par type et par destinataire : OF en retard, machine en arrêt prolongé, TRS sous seuil, lot bloqué scanné, habilitation expirée
- Escalade automatique si un aléa n'est pas traité dans un délai paramétrable
- Clôture d'un aléa avec : action réalisée, durée d'arrêt confirmée, responsable de clôture
- Journal des événements complet : historique de tous les aléas par machine, par période, par type — requêtable et exportable
- Notifications destinées aux bons rôles uniquement — un aléa machine n'alerte pas le Responsable Qualité, une NC n'alerte pas la maintenance

---

## 12. Rôles et contrôle d'accès (RBAC)

- Rôles minimum distincts avec périmètres fonctionnels séparés :
  - **Opérateur** : consultation et saisie sur ses OF assignés uniquement
  - **Superviseur de ligne** : visibilité sur tous les OF de son périmètre, gestion des priorités et des affectations
  - **Responsable de production / Directeur Industriel** : accès complet au suivi et aux indicateurs, sans accès à l'administration
  - **Responsable Qualité** : accès complet aux contrôles, NC, FAI, dérogations — sans accès à l'administration production
  - **Administrateur** : configuration du système, référentiels, gammes, utilisateurs, habilitations
- Cloisonnement strict : un opérateur ne peut pas consulter les OF d'une autre ligne ou d'un autre poste
- Authentification forte en bord de ligne : badge RFID, QR code ou PIN — pas de mot de passe complexe
- Signatures électroniques traçables pour les étapes critiques nécessitant une validation hiérarchique (FAI, dérogation, clôture NC)
- Journalisation complète et immuable de toutes les actions par utilisateur, rôle, horodatage

---

## 13. Conformité et auditabilité

- Audit trail complet et non altérable : qui a fait quoi, quand, depuis quel poste
- Conformité native EN9100 : traçabilité lot et série, gestion des NC, plans de contrôle, FAI intégrés au flux d'exécution — pas en surcouche
- Gestion des procédés spéciaux (Special Processes) : identification des opérations soumises à qualification NADCAP, blocage si l'opérateur ou l'équipement n'est pas qualifié
- Archivage long terme des données de production sans limite de durée, avec garantie d'intégrité
- Export du dossier de conformité complet (As-Built) en un clic, utilisable directement lors d'un audit

---

## 14. API, intégration et ouverture de la donnée

- API REST documentée (OpenAPI / Swagger) exposant l'intégralité des entités : OF, opérations, traçabilité, temps, TRS, aléas, NC, opérateurs, habilitations
- Authentification JWT sur tous les endpoints
- Événements temps réel (webhooks ou bus d'événements) pour les changements d'état critiques : OF terminé, aléa déclaré, lot bloqué, NC ouverte
- Import des OF et des gammes depuis l'ERP via fichier CSV ou appel API (format documenté et stable)
- Export des données de production vers l'ERP (quantités produites, rebuts, temps) via CSV ou API
- Documentation des schémas de données publique, versionnée et maintenue à jour

---

## 15. Déploiement, infrastructure et continuité de service

- Fonctionnement autonome On-Premise sans connexion internet requise pour la production
- Mises à jour de schéma de base de données sans interruption de service : migrations versionnées et réversibles, procédure de rollback documentée et testée
- Compatibilité avec les serveurs industriels standard (x86_64, 8 à 16 Go RAM)
- Mode dégradé : toutes les fonctions de saisie opérateur disponibles hors réseau, synchronisation automatique au rétablissement
- Accès multi-postes simultanés sans dégradation des performances (tablettes opérateurs + postes superviseurs + dashboards direction)
- Sauvegarde automatique des données avec politique de rétention configurable

---

## 16. Supervision et tableaux de bord

- Dashboard superviseur temps réel : statut de tous les OF en cours, statut de toutes les machines, alertes actives, TRS courant, aléas ouverts
- Dashboard direction : TRS par ligne et par période, avancement du plan de production, top causes d'arrêt, comparaison prévu vs réalisé
- Analyse des goulots d'étranglement : identification des postes limitant le flux sur une période donnée
- Historique de tous les indicateurs sur au minimum 24 mois glissants
- Reporting automatisé exportable (CSV, PDF) — planifiable hebdomadairement ou mensuellement
package i18n

// frCA is the French (Canada) translation.
// Empty fields fall back to enCA automatically.
// Translation progress: see README for current percentage.
var frCA = Strings{
	// ── App ──────────────────────────────────────────────────────────────
	AppTitle: "VideoTools",

	// ── Main Menu ────────────────────────────────────────────────────────
	MenuQueue:     "File d'attente",
	MenuBenchmark: "Référence",
	MenuResults:   "Résultats",
	MenuAbout:     "À propos / Soutien",

	// ── Module Names ─────────────────────────────────────────────────────
	ModuleConvert:     "Convertir",
	ModuleMerge:       "Fusionner",
	ModuleTrim:        "Rogner",
	ModuleFilters:     "Filtres",
	ModuleUpscale:     "Amélioration",
	ModuleEnhancement: "Optimisation",
	ModuleAudio:       "Audio",
	ModuleAuthor:      "Créer",
	ModuleRip:         "Extraire",
	ModuleBluRay:      "Blu-Ray",
	ModuleSubtitles:   "Sous-titres",
	ModuleThumbnail:   "Miniature",
	ModuleCompare:     "Comparer",
	ModuleInspect:     "Inspecter",
	ModulePlayer:      "Lecteur",
	ModuleSettings:    "Paramètres",

	// ── Module Category Labels ────────────────────────────────────────────
	CategoryConvert:  "Conversion",
	CategoryInspect:  "Inspection",
	CategoryDisc:     "Disque",
	CategoryPlayback: "Lecture",

	// ── Common Actions ────────────────────────────────────────────────────
	ActionGenerate:  "Générer",
	ActionCancel:    "Annuler",
	ActionSave:      "Enregistrer",
	ActionLoad:      "Charger",
	ActionReset:     "Réinitialiser",
	ActionBrowse:    "Parcourir",
	ActionOpen:      "Ouvrir",
	ActionClose:     "Fermer",
	ActionBack:      "Retour",
	ActionAdd:       "Ajouter",
	ActionRemove:    "Supprimer",
	ActionClear:     "Effacer",
	ActionClearAll:  "Tout effacer",
	ActionInstall:   "Installer",
	ActionUninstall: "Désinstaller",
	ActionStart:     "Démarrer",
	ActionStop:      "Arrêter",
	ActionPause:     "Pause",
	ActionResume:    "Reprendre",
	ActionDelete:    "Supprimer",
	ActionRefresh:   "Actualiser",
	ActionApply:     "Appliquer",
	ActionConfirm:   "Confirmer",
	ActionEdit:      "Modifier",
	ActionCopy:      "Copier",
	ActionExport:    "Exporter",
	ActionImport:    "Importer",

	// ── Common Labels ─────────────────────────────────────────────────────
	LabelInput:       "Entrée",
	LabelOutput:      "Sortie",
	LabelSource:      "Source",
	LabelDestination: "Destination",
	LabelFormat:      "Format",
	LabelQuality:     "Qualité",
	LabelResolution:  "Résolution",
	LabelBitrate:     "Débit binaire",
	LabelFrameRate:   "Fréquence d'images",
	LabelCodec:       "Codec",
	LabelAudio:       "Audio",
	LabelVideo:       "Vidéo",
	LabelSubtitles:   "Sous-titres",
	LabelDuration:    "Durée",
	LabelSize:        "Taille",
	LabelProgress:    "Progression",
	LabelStatus:      "État",
	LabelLanguage:    "Langue",
	LabelVersion:     "Version",
	LabelLicense:     "Licence",

	// ── Common Status ─────────────────────────────────────────────────────
	StatusReady:      "Prêt",
	StatusProcessing: "Traitement en cours...",
	StatusComplete:   "Terminé",
	StatusFailed:     "Échec",
	StatusCancelled:  "Annulé",
	StatusPending:    "En attente",
	StatusRunning:    "En cours...",
	StatusUpToDate:   "À jour",

	// ── Settings ──────────────────────────────────────────────────────────
	SettingsTitle:           "Paramètres",
	SettingsTabGeneral:      "Général",
	SettingsTabDependencies: "Dépendances",
	SettingsTabUpdates:      "Mises à jour",
	SettingsTabAbout:        "À propos",
	SettingsLanguage:        "Langue",
	SettingsLanguageScript:  "Système d'écriture",
	SettingsScriptSyllabics: "Syllabiques traditionnels",
	SettingsScriptLatin:     "Latin",
	SettingsTheme:           "Thème",
	SettingsOutputFolder:    "Dossier de sortie",

	// ── Updates ───────────────────────────────────────────────────────────
	UpdateCheckButton:      "Vérifier les mises à jour",
	UpdateInstall:          "Installer la mise à jour",
	UpdateInstallPatches:   "Installer les correctifs",
	UpdateHashMismatch:     "Discordance de hachage détectée :",
	UpdateHashCurrent:      "Actuel :",
	UpdateHashLatest:       "Dernier :",

	// ── Queue ─────────────────────────────────────────────────────────────
	QueueTitle:      "File d'attente",
	QueueEmpty:      "Aucune tâche en attente",
	QueueInProgress: "En cours",
	QueueCompleted:  "Terminé",
	QueueFailed:     "Échec",
	QueueJobRunning: "En cours...",
	QueueJobPending: "En attente",

	// ── History Sidebar ───────────────────────────────────────────────────
	HistoryTitle:     "HISTORIQUE",
	HistoryNoEntries: "Aucune entrée",

	// ── Convert ───────────────────────────────────────────────────────────
	ConvertDropPrompt:    "Déposez un fichier vidéo ici",
	ConvertOutputFormat:  "Format de sortie",
	ConvertHardwareAccel: "Accélération matérielle",

	// ── Thumbnail / Contact Sheet ─────────────────────────────────────────
	ThumbnailGenerateNow:  "Générer maintenant",
	ThumbnailContactSheet: "Planche contact",
	ThumbnailColumns:      "Colonnes",
	ThumbnailRows:         "Rangées",
	ThumbnailOutputFolder: "Dossier de sortie",

	// ── About ─────────────────────────────────────────────────────────────
	AboutTitle:       "À propos de VideoTools",
	AboutDescription: "Une boîte à outils vidéo native conçue pour la vitesse et la simplicité.",
	AboutLicense:     "Licence",
	AboutSupport:     "Soutien",

	// ── Errors ────────────────────────────────────────────────────────────
	ErrFileNotFound:   "Fichier introuvable : %s",
	ErrNoOutputFolder: "Aucun dossier de sortie sélectionné.",
	ErrFFmpegMissing:  "FFmpeg n'est pas installé ou est introuvable.",
	ErrProcessFailed:  "Échec du processus : %s",
	ErrConfigLoad:     "Impossible de charger la configuration : %s",
	ErrConfigSave:     "Impossible d'enregistrer la configuration : %s",
}

func init() {
	register("fr-CA", frCA)
}

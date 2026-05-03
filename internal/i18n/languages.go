package i18n

// allLanguages is the ordered list of supported languages shown in Settings.
var allLanguages = []Language{
	{
		Code:        "en-CA",
		EnglishName: "English (Canada)",
		NativeName:  "English (Canada)",
		Font:        "mono",
		Flag:        "FLAG_canada.svg",
	},
	{
		Code:        "fr-CA",
		EnglishName: "French (Canada)",
		NativeName:  "Français (Canada)",
		Font:        "mono",
		Flag:        "FLAG_quebec.svg",
	},
	{
		Code:        "iu",
		EnglishName: "Inuktitut",
		NativeName:  "ᐃᓄᒃᑎᑐᑦ",
		Font:        "aboriginal",
		Flag:        "FLAG_nunavut.svg",
	},
}

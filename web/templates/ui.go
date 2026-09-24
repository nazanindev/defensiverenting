package templates

import (
	"fmt"
	"time"
)

// UI chrome strings for the language-routed pages (ADR-007 D2's content
// routes: guide (playbook), jurisdiction hub, topic hub, 404, and the shared footer).
// Statements are translated by the drafting pipeline; this table is for the
// furniture around them, so a Spanish page never wraps Spanish law in English
// chrome. The Spanish register is usted, matching the translated statements.
//
// Lookup rules: a missing language falls back to English; a missing key
// renders the key itself, which is deliberately ugly so it cannot ship
// unnoticed. English-only chrome (/, /search, the reference layer) keeps its
// strings inline in the templates; only strings a Spanish page renders live
// here.
var uiStrings = map[string]map[string]string{
	// Shared header and footer
	"search-placeholder": {"en": "Search by situation…", "es": "Busque su situación…"},
	"search":             {"en": "Search", "es": "Buscar"},
	"tagline":            {"en": "Tenant law in plain language", "es": "La ley para inquilinos, en palabras simples"},
	"choose-location":    {"en": "Choose your location", "es": "Elija su lugar"},
	"your-location":      {"en": "Your location", "es": "Su lugar"},
	"header-terms":       {"en": "Legal terms", "es": "Términos legales"},
	"footer-promise":     {"en": "Every statement links to the law it comes from.", "es": "Cada afirmación enlaza a la ley de donde viene."},
	"footer-guides":      {"en": "Guides", "es": "Guías"},
	"footer-about-us":    {"en": "About us", "es": "Quiénes somos"},
	"footer-disclaimer": {
		"en": "This site gives general information. It is not a substitute for a lawyer.",
		"es": "Este sitio ofrece información general. No reemplaza a un abogado.",
	},
	"not-legal-advice": {"en": "Not legal advice.", "es": "Esto no es asesoría legal."},
	"footer-locations": {"en": "All locations", "es": "Todos los lugares"},
	"footer-about":     {"en": "About", "es": "Acerca de"},
	"footer-support":   {"en": "Support this project", "es": "Apoye este proyecto"},
	"footer-editorial": {"en": "Editorial standards", "es": "Normas editoriales"},
	"footer-report":    {"en": "Report a problem", "es": "Reporte un problema"},
	"footer-contact":   {"en": "Contact us", "es": "Contacto"},
	"home":             {"en": "Home", "es": "Inicio"},

	// Playbook page
	"all-topics":    {"en": "← All topics", "es": "← Todos los temas"},
	"nationwide":    {"en": "Nationwide", "es": "Todo el país"},
	"reviewed-by":   {"en": "Last reviewed by", "es": "Última revisión por"},
	"last-verified": {"en": "Last verified", "es": "Verificado por última vez el"},
	"need-help-now": {"en": "Need help now?", "es": "¿Necesita ayuda ahora?"},
	"in":            {"en": "in", "es": "en"},
	"help-bar-tail": {"en": "legal aid, rent assistance, and who to call", "es": "ayuda legal, ayuda con la renta y a quién llamar"},
	"page-disclaimer": {
		"en": "Every statement below links to the law it comes from. Read the source before you act on it.",
		"es": "Cada afirmación de abajo enlaza a la ley de donde viene. Lea la fuente antes de actuar.",
	},
	"page-disclaimer-help": {
		"en": "If you need legal help, contact a legal aid office near you.",
		"es": "Si necesita ayuda legal, contacte a la organización de ayuda legal de su área.",
	},
	"sources":           {"en": "Sources", "es": "Fuentes"},
	"sources-checked":   {"en": "Sources checked", "es": "Fuentes verificadas el"},
	"details-checked":   {"en": "Details checked", "es": "Datos verificados el"},
	"rule-depends":      {"en": "This rule depends on where you live.", "es": "Esta regla depende de dónde vive."},
	"rule-everywhere":   {"en": "See the rule in every place we cover.", "es": "Vea la regla en cada lugar que cubrimos."},
	"full-guides":       {"en": "We have full guides on this.", "es": "Tenemos guías completas sobre esto."},
	"see-guides":        {"en": "See our %s guides.", "es": "Vea nuestras guías de %s."},
	"no-statements":     {"en": "This guide has no statements yet.", "es": "Esta guía aún no tiene contenido."},
	"report-question":   {"en": "Is something on this page wrong or out of date?", "es": "¿Algo en esta página está mal o desactualizado?"},
	"report-tell-us":    {"en": "Tell us", "es": "Avísenos"},
	"more-nationwide":   {"en": "More nationwide tenant rights guides", "es": "Más guías nacionales de derechos del inquilino"},
	"more-rights-in":    {"en": "More tenant rights in %s", "es": "Más derechos del inquilino en %s"},
	"where-do-you-rent": {"en": "Where do you rent?", "es": "¿Dónde renta usted?"},
	"law-is-local": {
		"en": "Most tenant law is set by your state and city. Pick your place for the exact rules and deadlines. The guide below covers what applies everywhere.",
		"es": "La mayoría de las leyes de inquilinos las fija su estado y su ciudad. Elija su lugar para ver las reglas y los plazos exactos. La guía de abajo cubre lo que aplica en todo el país.",
	},
	"topic-elsewhere":  {"en": "%s in other cities", "es": "%s en otras ciudades"},
	"all-cities-topic": {"en": "All cities for this topic →", "es": "Todas las ciudades para este tema →"},

	// Jurisdiction hub
	"whats-your-situation": {"en": "What’s your situation?", "es": "¿Cuál es su situación?"},
	"hub-lede": {
		"en": "Search your situation, or pick a topic below. Every statement links to the law it comes from.",
		"es": "Busque su situación o elija un tema abajo. Cada afirmación enlaza a la ley de donde viene.",
	},
	"hub-search-placeholder": {"en": "e.g. heat stopped working…", "es": "por ejemplo: la calefacción no funciona…"},
	"nationwide-guides":      {"en": "Nationwide guides", "es": "Guías nacionales"},
	"statewide-rules":        {"en": "For all of %s", "es": "Para todo %s"},
	"place-law":              {"en": "%s law", "es": "Ley de %s"},
	"pick-a-topic":           {"en": "Or pick a topic", "es": "Elija un tema"},
	"cities-in":              {"en": "Cities in %s", "es": "Ciudades en %s"},
	"ordinances-stack": {
		"en": "City rules add to %s law. If we cover your city, start there.",
		"es": "Las reglas de la ciudad se suman a la ley de %s. Si cubrimos su ciudad, empiece por ahí.",
	},
	"no-playbooks-yet":  {"en": "No guides for %s yet.", "es": "Aún no hay guías para %s."},
	"see-all-locations": {"en": "See all locations →", "es": "Vea todos los lugares →"},

	// Topic hub
	"topic-hub-lede": {
		"en": "The rules on this topic depend on your state and city. Pick your place for the guide that applies to you.",
		"es": "Las reglas de este tema dependen de su estado y su ciudad. Elija su lugar para ver la guía que le aplica.",
	},
	"choose-your-city": {"en": "Choose your city", "es": "Elija su ciudad"},
	"covered-count":    {"en": "(%d covered)", "es": "(%d cubiertas)"},
	"other-group":      {"en": "Other", "es": "Otros"},
	"dont-see-city": {
		"en": "Don’t see your city? We add new places one at a time. Your state’s guide still applies to you.",
		"es": "¿No ve su ciudad? Agregamos lugares nuevos uno por uno. La guía de su estado igual le aplica.",
	},
	"statewide-guides": {"en": "Statewide guides", "es": "Guías estatales"},
	"national-guide":   {"en": "Nationwide guide", "es": "Guía nacional"},
	"national-applies": {
		"en": "This guide explains the rules that apply in every state. Your state and city can add more protections on top.",
		"es": "Esta guía explica las reglas que aplican en todos los estados. Su estado y su ciudad pueden sumar más protecciones.",
	},

	// 404
	"nf-no-guide-for":    {"en": "No %s guide for %s yet", "es": "Aún no hay guía de %s para %s"},
	"nf-other-guides":    {"en": "We have other guides for %s, but not this one yet.", "es": "Tenemos otras guías para %s, pero esta todavía no."},
	"nf-no-topic-guides": {"en": "No %s guides yet", "es": "Aún no hay guías de %s"},
	"nf-none-published":  {"en": "No guides are published for this topic yet.", "es": "Aún no hay guías publicadas para este tema."},
	"nf-uncovered":       {"en": "We don’t cover this place yet", "es": "Aún no cubrimos este lugar"},
	"nf-one-at-a-time":   {"en": "RenterLaw adds new places one at a time.", "es": "RenterLaw agrega lugares nuevos uno por uno."},
	"nf-not-found":       {"en": "We can’t find that page", "es": "No encontramos esa página"},
	"nf-moved":           {"en": "The link may be wrong, or the page may have moved.", "es": "El enlace puede estar mal, o la página pudo haber cambiado de lugar."},
	"nf-nearest-covers":  {"en": "The %s guide covers this", "es": "La guía de %s cubre esto"},
	"nf-nearest-applies": {"en": "%s law applies in %s. Start there until we write the %s guide.", "es": "La ley de %s aplica en %s. Empiece ahí mientras escribimos la guía de %s."},
	"nf-topic-in":        {"en": "%s in %s", "es": "%s en %s"},
	"nf-where-instead":   {"en": "Where to go instead", "es": "A dónde ir mientras tanto"},
	"nf-everything-for":  {"en": "Everything we have for %s", "es": "Todo lo que tenemos para %s"},
	"nf-all-places":      {"en": "All the places we cover", "es": "Todos los lugares que cubrimos"},
	"nf-search-home":     {"en": "Search from the homepage", "es": "Busque desde la página principal"},
	"nf-why-missing":     {"en": "Why some pages are missing", "es": "Por qué faltan algunas páginas"},
	"nf-why-body": {
		"en": "A person checks every citation before a guide is published. That is why coverage grows one place and one topic at a time. If we do not cover your city yet, your state’s guide and the nationwide guides still apply to you.",
		"es": "Una persona verifica cada cita antes de publicar una guía. Por eso la cobertura crece lugar por lugar y tema por tema. Si aún no cubrimos su ciudad, la guía de su estado y las guías nacionales igual aplican para usted.",
	},
	"nf-ask-cover": {"en": "Ask us to cover your city →", "es": "Pídanos cubrir su ciudad →"},
}

var esMonths = [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio",
	"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

// UIString returns the chrome string for a language, falling back to English,
// then to the bare key (ugly on purpose: a typo'd key must be visible).
func UIString(lang, key string) string {
	byLang, ok := uiStrings[key]
	if !ok {
		return key
	}
	if s, ok := byLang[lang]; ok {
		return s
	}
	return byLang["en"]
}

// UIStringf is UIString for strings with fmt verbs.
func UIStringf(lang, key string, args ...any) string {
	return fmt.Sprintf(UIString(lang, key), args...)
}

// UIDate renders a date the way the page's language writes one.
func UIDate(lang string, t time.Time) string {
	if lang == "es" {
		return fmt.Sprintf("%d de %s de %d", t.Day(), esMonths[t.Month()-1], t.Year())
	}
	return t.Format("January 2, 2006")
}

// pageLang reports the language a page renders in, for templates shared
// across page types (the footer, the header search). English-only pages fall
// through to "en" without needing a language field of their own.
// Header is what the shared site header needs from a page. Most pages need
// nothing but their language; see headerFor.
type Header struct {
	Lang string
	// Scope is the jurisdiction slug the header search is scoped to, so a
	// search from a Boston page searches Boston. Empty searches everywhere.
	Scope string
	// NoSearch drops the header search on a page that is itself a search.
	NoSearch bool
	// SignedIn is known server-side only on the uncached account page. On
	// every other page the header script flips "Sign in" from the cookie.
	SignedIn bool
}

// headerFor derives the header's inputs from a page, the way pageLang derives
// its language, so each template calls one line instead of repeating the
// header's markup.
func headerFor(v any) Header {
	h := Header{Lang: pageLang(v)}
	switch p := v.(type) {
	case PlaybookPage:
		h.Scope = p.Jurisdiction.Slug
	case JurisdictionPage:
		h.Scope = p.Jurisdiction.Slug
	case SearchPage:
		h.NoSearch = true
	case AccountPage:
		h.SignedIn = p.SignedIn
	}
	return h
}

func pageLang(v any) string {
	switch p := v.(type) {
	case PlaybookPage:
		if p.Playbook.Language != "" {
			return p.Playbook.Language
		}
	case JurisdictionPage:
		if p.Language != "" {
			return p.Language
		}
	case TopicHubPage:
		if p.Language != "" {
			return p.Language
		}
	case NotFoundPage:
		if p.Language != "" {
			return p.Language
		}
	}
	return "en"
}

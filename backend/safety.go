package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

// EmergencyCategory defines the classification of detected medical emergency
type EmergencyCategory string

const (
	CategoryChestPain           EmergencyCategory = "chest_pain"
	CategoryDifficultyBreathing EmergencyCategory = "difficulty_breathing"
	CategorySevereBleeding      EmergencyCategory = "severe_bleeding"
	CategoryUnconsciousness     EmergencyCategory = "unconsciousness_fainting"
	CategorySeizure             EmergencyCategory = "seizure"
	CategoryStroke              EmergencyCategory = "stroke"
	CategoryChoking             EmergencyCategory = "choking"
	CategoryOverdosePoisoning   EmergencyCategory = "overdose_poisoning"
	CategoryPregnancyBleeding   EmergencyCategory = "pregnancy_bleeding"
	CategorySelfHarm            EmergencyCategory = "self_harm"
)

var nonAlnumRegex = regexp.MustCompile(`[^a-z0-9\s]+`)

// NormaliseSafetyInput lowercases, strips punctuation, and normalises whitespace
func NormaliseSafetyInput(text string) string {
	lower := strings.ToLower(text)
	cleaned := nonAlnumRegex.ReplaceAllString(lower, " ")
	fields := strings.Fields(cleaned)
	return strings.Join(fields, " ")
}

// EmergencyPhrases maps categories to matching triggers in English, Swahili, and Sheng
// NOTE FOR PR REVIEW: Swahili & Sheng phrases to be reviewed by a native Kenyan speaker and clinician.
var EmergencyPhrases = map[EmergencyCategory][]string{
	CategoryChestPain: {
		// English
		"chest pain", "pain in chest", "chest pains", "tight chest", "chest tightness",
		"chest pressure", "pressure in chest", "heart attack", "crushing chest",
		"heavy chest", "pain in my chest", "aching chest", "sharp chest pain",
		"chest hurts", "chest is hurting", "chest discomfort",
		// Swahili & Sheng
		"maumivu ya kifua", "kifua kinauma", "inauma kifua", "kifua kubana",
		"kifua kinabana", "kifua inabana", "kifua kuuma", "kifua kinachoma",
		"choma kifua", "chest inaniuma", "kifua tight", "chest inabana",
		"maumivu ya kifuwa", "kifuwa kinauma", "chesti inauma", "kifua kuanza kuuma",
	},
	CategoryDifficultyBreathing: {
		// English
		"difficulty breathing", "cannot breathe", "can't breathe", "cant breathe",
		"shortness of breath", "struggling to breathe", "breathless", "suffocating",
		"gasping for air", "trouble breathing", "hard to breathe", "unable to breathe",
		"running out of breath", "choking for air", "lack of air", "short of breath",
		"gasping", "breathlessness",
		// Swahili & Sheng
		"siwezi kupumua", "kupumua kwa shida", "anapumua kwa shida", "ninapumua kwa shida",
		"nashindwa kupumua", "anashindwa kupumua", "pumzi imekata",
		"pumzi shida", "kukosa pumzi", "pumzi ngumu", "kushindwa kupumua",
		"shida ya kupumua", "pumzi inakosa", "pumzi kuisha", "pumzi inakata",
		"pumzi imeisha", "siwezi pumua", "kupumua ngumu", "pumzi mbaya",
		"pumua kwa shida", "siwezi ku breath", "siwezi kubreathe",
	},
	CategorySevereBleeding: {
		// English
		"severe bleeding", "bleeding heavily", "heavy bleeding", "gushing blood",
		"losing so much blood", "blood wont stop", "blood won't stop", "profuse bleeding",
		"bleeding profusely", "blood pouring", "bleeding uncontrollably", "spurting blood",
		"hemorrhage", "haemorrhage", "bleeding too much", "lots of blood",
		// Swahili & Sheng
		"kutokwa na damu nyingi", "damu inatoka sana", "damu nyingi", "damu haisimami",
		"damu inamwagika", "kutoka damu nyingi", "damu mingi", "damu haikomi",
		"kutokwa damu sana", "damu mob", "bleed mob", "damu inatoka mob",
		"damu mob sana", "damu nyiingi", "damu inagush", "kutoka damu mob",
	},
	CategoryUnconsciousness: {
		// English
		"unconscious", "passed out", "blacked out", "fainted", "collapsed",
		"unresponsive", "fainting", "fell unconscious", "lost consciousness",
		"losing consciousness", "passedout", "blackout", "not waking up",
		"cannot wake up", "cant wake up", "unresponsiveness",
		// Swahili & Sheng
		"amepoteza fahamu", "nimezimia", "amezimia", "kupoteza fahamu",
		"kuzimia", "kuanguka bila fahamu", "kuzirai", "amezirai",
		"hapumui wala kuamka", "hazinduki", "amecollapse", "amezimia ghafla",
		"nimecollapse", "ame pass out", "ku pass out", "nime pass out",
	},
	CategorySeizure: {
		// English
		"seizure", "seizures", "convulsion", "convulsions", "convulsing",
		"epileptic fit", "epilepsy fit", "having a fit", "shaking uncontrollably",
		"jerking uncontrollably", "grand mal", "seizing",
		// Swahili & Sheng
		"kifafa", "degedege", "mshtuko wa kifafa", "kutingizika mwili",
		"kutetemeka mwili ghafla", "kushikwa na kifafa", "kushake bila control",
		"fit ya kifafa", "kifafa kimeshika", "mtoto anastuka",
	},
	CategoryStroke: {
		// English
		"stroke", "face drooping", "drooping face", "face droop", "slurred speech",
		"slurring words", "arm weakness", "weakness on one side", "paralyzed on one side",
		"paralysis on one side", "sudden numbness", "loss of speech", "cannot speak suddenly",
		// Swahili & Sheng
		"kiharusi", "kupooza", "mdomo kwenda upande", "kupoteza uwezo wa kuongea",
		"mkono umekufa ganzi", "upande mmoja wa mwili umepooza", "kupooza mwili",
		"stroke attack", "mdomo imepinda", "side moja imekufa ganzi",
	},
	CategoryChoking: {
		// English
		"choking", "choked", "airway blocked", "blocked airway", "object stuck in throat",
		"food stuck in throat", "bone stuck in throat", "something stuck in throat",
		"swallowed a coin", "throat blocked", "choking on food",
		// Swahili & Sheng
		"kaba koo", "kukabwa", "kunyongwa na chakula", "kukabwa na chakula",
		"chakula kimekwama kooni", "mfupa umekwama kooni", "kitu kimekwama kooni",
		"kukwama kooni", "kushindwa kumeza kwa kukabwa", "nimekwamwa na chakula",
		"kukwama kwa throat", "food imekwama",
	},
	CategoryOverdosePoisoning: {
		// English
		"overdose", "overdosed", "poisoning", "poisoned", "swallowed poison",
		"drank poison", "toxic ingestion", "swallowed chemicals", "swallowed bleach",
		"drank bleach", "swallowed pills", "took too many pills", "drank pesticide",
		"ingested poison", "overdosing",
		// Swahili & Sheng
		"sumu", "kunywa sumu", "kumeza sumu", "sumu ya panya", "amenywa sumu",
		"ametumia dawa kupita kiasi", "kunywa dawa nyingi kupita kiasi",
		"overdose ya dawa", "ameoverdose", "nimeoverdose", "kunywa dawa mingi",
	},
	CategoryPregnancyBleeding: {
		// English
		"pregnancy bleeding", "bleeding while pregnant", "pregnant and bleeding",
		"bleeding in pregnancy", "miscarriage", "miscarrying", "bleeding pregnancy",
		"cramping and bleeding pregnant", "spotting heavily pregnant", "bleeding with pregnancy",
		// Swahili & Sheng
		"kutokwa na damu mimba", "damu wakati wa ujauzito", "damu na nina mimba",
		"mimba inatoka", "kutoka mimba", "damu ya mimba", "kuharibika kwa mimba",
		"mjamzito anatokwa na damu", "mimba inatoka damu", "ball inatoka damu",
		"damu kwa ball", "kutoka ball",
	},
	CategorySelfHarm: {
		// English
		"suicide", "suicidal", "kill myself", "want to kill myself", "going to kill myself",
		"end my life", "want to end my life", "want to die", "better off dead",
		"hurt myself", "want to hurt myself", "self harm", "self-harm", "cutting myself",
		"cut my wrists", "overdose to die", "commit suicide", "take my own life",
		// Swahili & Sheng
		"kujiua", "nataka kujiua", "kujitoa uhai", "kujiumiza", "sitaki kuishi",
		"nataka kufa", "kujinyonga", "kujitupa", "kujikata", "kukatiza maisha yangu",
		"kujimaliza", "nataka kujimaliza", "kuenda zangu milele", "kuji kill",
	},
}

// DetectEmergency determines whether user text contains red-flag emergency symptoms
func DetectEmergency(rawText string) (bool, EmergencyCategory) {
	norm := NormaliseSafetyInput(rawText)
	if norm == "" {
		return false, ""
	}

	padded := " " + norm + " "

	// Priority: Self-harm and specific conditions (pregnancy bleeding, choking, poisoning) first,
	// followed by acute organ-system emergencies.
	categories := []EmergencyCategory{
		CategorySelfHarm,
		CategoryPregnancyBleeding,
		CategoryChoking,
		CategoryOverdosePoisoning,
		CategoryStroke,
		CategorySeizure,
		CategoryChestPain,
		CategoryDifficultyBreathing,
		CategorySevereBleeding,
		CategoryUnconsciousness,
	}

	for _, cat := range categories {
		for _, phrase := range EmergencyPhrases[cat] {
			normPhrase := NormaliseSafetyInput(phrase)
			if normPhrase == "" {
				continue
			}
			// Exact substring in space-padded normalized text
			if strings.Contains(padded, " "+normPhrase+" ") {
				return true, cat
			}
			// Substring check for compound words/misspellings
			if len(normPhrase) > 4 && strings.Contains(norm, normPhrase) {
				return true, cat
			}
		}
	}

	return false, ""
}

// EmergencyResponse builds the fixed template response (never LLM-generated)
func EmergencyResponse(tenant Tenant, category EmergencyCategory) string {
	if category == CategorySelfHarm {
		return "If you are feeling overwhelmed, hopeless, or having thoughts of self-harm or suicide, please know that you are not alone and support is available right now. Please reach out immediately to a trusted person, visit the nearest hospital emergency room, or call emergency services at 999 or 112. In Kenya, you can also reach Befrienders Kenya at +254 722 178 177 for free, confidential support."
	}

	if tenant.EmergencyMessage != "" {
		return tenant.EmergencyMessage
	}

	emergencyNum := "999 or 112"
	if tenant.EmergencyNumber != "" {
		emergencyNum = fmt.Sprintf("%s (or 999 / 112)", tenant.EmergencyNumber)
	}

	return fmt.Sprintf("EMERGENCY NOTICE: If you or someone with you is experiencing a medical emergency, please call emergency services immediately at %s, or go to the nearest emergency department right now. Please do not wait for an online appointment booking.", emergencyNum)
}

// LogEmergencyDetected records an emergency event without logging any patient message text
func LogEmergencyDetected(tenantSlug, sessionID string, category EmergencyCategory) {
	log.Printf("[EMERGENCY_DETECTED] tenant=%s session_id=%s category=%s timestamp=%s",
		tenantSlug, sessionID, string(category), time.Now().UTC().Format(time.RFC3339))
}


package main

import (
	"strings"
	"testing"
)

func TestDetectEmergency(t *testing.T) {
	emergencyCases := []struct {
		name     string
		input    string
		expected EmergencyCategory
	}{
		// 1. Chest pain
		{"Chest pain English 1", "I have severe chest pain please help", CategoryChestPain},
		{"Chest pain English 2", "my chest hurts really bad and feels tight", CategoryChestPain},
		{"Chest pain English 3", "I think I am having a heart attack", CategoryChestPain},
		{"Chest pain Swahili 1", "Nina maumivu ya kifua makali sana tangu asubuhi", CategoryChestPain},
		{"Chest pain Swahili 2", "kifua kinauma vibaya siwezi kulala", CategoryChestPain},
		{"Chest pain Sheng", "chest inaniuma sana niaje", CategoryChestPain},

		// 2. Difficulty breathing
		{"Difficulty breathing English 1", "I cannot breathe properly right now", CategoryDifficultyBreathing},
		{"Difficulty breathing English 2", "can't breathe and feeling suffocating", CategoryDifficultyBreathing},
		{"Difficulty breathing English 3", "experiencing severe shortness of breath", CategoryDifficultyBreathing},
		{"Difficulty breathing Swahili 1", "siwezi kupumua nisaidieni tafadhali", CategoryDifficultyBreathing},
		{"Difficulty breathing Swahili 2", "mgonjwa anapumua kwa shida sana", CategoryDifficultyBreathing},
		{"Difficulty breathing Swahili 3", "pumzi imekata kabisa hawezi kuongea", CategoryDifficultyBreathing},
		{"Difficulty breathing Sheng", "siwezi kubreathe bana", CategoryDifficultyBreathing},

		// 3. Severe bleeding
		{"Severe bleeding English 1", "There is severe bleeding from the wound", CategorySevereBleeding},
		{"Severe bleeding English 2", "gushing blood and it won't stop", CategorySevereBleeding},
		{"Severe bleeding English 3", "losing so much blood very quickly", CategorySevereBleeding},
		{"Severe bleeding Swahili 1", "anatokwa na damu nyingi mguuni", CategorySevereBleeding},
		{"Severe bleeding Swahili 2", "damu inatoka sana haisimami ata kidogo", CategorySevereBleeding},
		{"Severe bleeding Sheng", "damu mob inamwagika baada ya ajali", CategorySevereBleeding},

		// 4. Unconsciousness / Fainting
		{"Unconscious English 1", "my brother passed out suddenly in the living room", CategoryUnconsciousness},
		{"Unconscious English 2", "he fell unconscious on the floor and is not waking up", CategoryUnconsciousness},
		{"Unconscious English 3", "patient is completely unresponsive", CategoryUnconsciousness},
		{"Unconscious Swahili 1", "amepoteza fahamu ghafla haamki", CategoryUnconsciousness},
		{"Unconscious Swahili 2", "nimezimia mara mbili leo asubuhi", CategoryUnconsciousness},
		{"Unconscious Swahili 3", "mama amezirai hafungui macho", CategoryUnconsciousness},
		{"Unconscious Sheng", "buda ame pass out hapa", CategoryUnconsciousness},

		// 5. Seizure
		{"Seizure English 1", "he is having a seizure right now on the couch", CategorySeizure},
		{"Seizure English 2", "she is convulsing and shaking uncontrollably", CategorySeizure},
		{"Seizure Swahili 1", "mtoto ameshikwa na degedege kali", CategorySeizure},
		{"Seizure Swahili 2", "ana mshtuko wa kifafa na mapovu", CategorySeizure},
		{"Seizure Swahili 3", "kushikwa na kifafa ghafla", CategorySeizure},
		{"Seizure Sheng", "anashake bila control ameshikwa na fit ya kifafa", CategorySeizure},

		// 6. Stroke
		{"Stroke English 1", "her face is drooping and she has arm weakness", CategoryStroke},
		{"Stroke English 2", "sudden slurred speech and cannot lift left arm", CategoryStroke},
		{"Stroke Swahili 1", "babu ana dalili za kiharusi upande mmoja", CategoryStroke},
		{"Stroke Swahili 2", "upande mmoja wa mwili umepooza hawezi kunyanyua mkono", CategoryStroke},
		{"Stroke Sheng", "side moja imekufa ganzi na mdomo imepinda", CategoryStroke},

		// 7. Choking
		{"Choking English 1", "my child is choking on an apple piece", CategoryChoking},
		{"Choking English 2", "swallowed a coin and cannot breathe airway blocked", CategoryChoking},
		{"Choking Swahili 1", "chakula kimekwama kooni hawezi kupumua", CategoryChoking},
		{"Choking Swahili 2", "mfupa umekwama kooni ananyongwa", CategoryChoking},
		{"Choking Sheng", "food imekwama kwa throat bana nisaidie", CategoryChoking},

		// 8. Overdose / Poisoning
		{"Overdose English 1", "he took an overdose of prescription pills", CategoryOverdosePoisoning},
		{"Overdose English 2", "the toddler swallowed bleach from the cabinet", CategoryOverdosePoisoning},
		{"Overdose English 3", "ingested toxic chemicals and drank poison", CategoryOverdosePoisoning},
		{"Overdose Swahili 1", "amekunywa sumu ya panya kimakosa", CategoryOverdosePoisoning},
		{"Overdose Swahili 2", "ametumia dawa kupita kiasi anatapika", CategoryOverdosePoisoning},
		{"Overdose Sheng", "ameoverdose dawa za usingizi", CategoryOverdosePoisoning},

		// 9. Pregnancy bleeding
		{"Pregnancy bleeding English 1", "I am pregnant and bleeding heavily with pain", CategoryPregnancyBleeding},
		{"Pregnancy bleeding English 2", "having severe pregnancy bleeding at 12 weeks", CategoryPregnancyBleeding},
		{"Pregnancy bleeding English 3", "scared I am miscarrying because of heavy blood", CategoryPregnancyBleeding},
		{"Pregnancy bleeding Swahili 1", "mjamzito anatokwa na damu nyingi", CategoryPregnancyBleeding},
		{"Pregnancy bleeding Swahili 2", "nina mimba na ninatokwa na damu ya mimba", CategoryPregnancyBleeding},
		{"Pregnancy bleeding Sheng", "ball inatoka damu nisaidie haraka", CategoryPregnancyBleeding},

		// 10. Self-harm / Suicidal
		{"Self harm English 1", "I want to kill myself I can't take this anymore", CategorySelfHarm},
		{"Self harm English 2", "having thoughts of wanting to end my life", CategorySelfHarm},
		{"Self harm English 3", "planning self-harm and want to die", CategorySelfHarm},
		{"Self harm Swahili 1", "nataka kujiua nimechoka kabisa na maisha", CategorySelfHarm},
		{"Self harm Swahili 2", "nina mpango wa kujitoa uhai leo usiku", CategorySelfHarm},
		{"Self harm Sheng", "nataka kujimaliza nishachoka", CategorySelfHarm},
	}

	for _, tc := range emergencyCases {
		t.Run(tc.name, func(t *testing.T) {
			isEm, cat := DetectEmergency(tc.input)
			if !isEm {
				t.Fatalf("Expected emergency detection for input: %q, but got none", tc.input)
			}
			if cat != tc.expected {
				t.Errorf("Expected category %q for input %q, got %q", tc.expected, tc.input, cat)
			}
		})
	}
}

func TestNormalBookingMessagesDoNotTriggerEmergency(t *testing.T) {
	benignMessages := []string{
		"Book me with Dr. Kim for tomorrow at 10am",
		"I need an appointment with Dr. Wangechi next Tuesday at 2pm",
		"Hello, my name is John Kamau, I would like a routine dental checkup",
		"Can I reschedule my appointment to next Monday at 11:00?",
		"What are your consultation fees and operating hours?",
		"I want to bring my son for his routine MMR vaccination on Friday",
		"Is Dr. Mercy available for general consultation this Thursday?",
		"Looking for an eye checkup consultation next week",
		"I would like to book a blood pressure check with a nurse",
		"Need a medical certificate for my driving license renewal",
		"Habari, nataka kuonana na daktari kesho asubuhi saa nne",
		"Ninataka miadi ya kuonana na daktari Ijumaa saa kumi jioni",
		"Nataka kupima macho na meno tafadhali, naitwa Wanjiku",
		"Nahitaji kuona Dr. Mercy wiki ijayo kwa uchunguzi wa kawaida",
		"My tooth has a mild cavity, can I see the dentist on Wednesday?",
		"Need a skin consultation for minor rash",
	}

	for _, msg := range benignMessages {
		t.Run(msg, func(t *testing.T) {
			isEm, cat := DetectEmergency(msg)
			if isEm {
				t.Fatalf("False positive emergency trigger on benign message %q (detected category: %s)", msg, cat)
			}
		})
	}
}

func TestEmergencyResponseTemplates(t *testing.T) {
	defaultTenant := Tenant{ID: 1, Name: "Nairobi Health", Slug: "default"}

	// 1. General medical emergency template
	resp := EmergencyResponse(defaultTenant, CategoryChestPain)
	if !strings.Contains(resp, "999") || !strings.Contains(resp, "112") {
		t.Errorf("Expected default numbers 999 and 112 in emergency response, got: %s", resp)
	}
	if !strings.Contains(resp, "emergency department") {
		t.Errorf("Expected emergency department guidance in response, got: %s", resp)
	}

	// 2. Self-harm supportive template
	selfHarmResp := EmergencyResponse(defaultTenant, CategorySelfHarm)
	if !strings.Contains(selfHarmResp, "Befrienders Kenya") || !strings.Contains(selfHarmResp, "+254 722 178 177") {
		t.Errorf("Expected Befrienders Kenya contact in self-harm response, got: %s", selfHarmResp)
	}
	if !strings.Contains(selfHarmResp, "not alone") {
		t.Errorf("Expected supportive phrasing in self-harm response, got: %s", selfHarmResp)
	}

	// 3. Tenant custom emergency number
	customNumTenant := Tenant{
		ID:              2,
		Name:            "Karen Clinic",
		Slug:            "karen",
		EmergencyNumber: "+254 700 000 000",
	}
	customResp := EmergencyResponse(customNumTenant, CategorySevereBleeding)
	if !strings.Contains(customResp, "+254 700 000 000") {
		t.Errorf("Expected custom emergency number in response, got: %s", customResp)
	}

	// 4. Tenant custom emergency message
	customMsgTenant := Tenant{
		ID:               3,
		Name:             "Mombasa Hospital",
		Slug:             "mombasa",
		EmergencyMessage: "Custom Emergency Notice: Please rush to Mombasa Hospital ER at Ground Floor.",
	}
	customMsgResp := EmergencyResponse(customMsgTenant, CategoryDifficultyBreathing)
	if customMsgResp != "Custom Emergency Notice: Please rush to Mombasa Hospital ER at Ground Floor." {
		t.Errorf("Expected custom emergency message, got: %s", customMsgResp)
	}
}


# Topic map

The one piece of context every drafting and review agent gets beyond its own page (ADR-026, split by area in ADR-028 D8). An agent reads the playbook table and the rules below, then only the concept section for its page's area.

Four kinds of page (ADR-028 D1):

- A **playbook** answers one *situation* a renter is in, as steps in the order they act. Its title is the renter's search question. Its statements sit under the topic's fixed **stages**.
- A **rules page** answers one place's *rules* for one topic, one statement per concept, each under the concept's question. Its title is "{Topic} Rules in {Place}: What Does the Law Say?".
- A **checklist** is one national page of things to check, linking to concepts for the numbers.
- A **concept** is one informational question ("how much can my landlord charge for a deposit?"). Concept pages are assembled from tagged statements across places.

A statement is one informative claim, tagged with the concept whose question it answers. It sits on a playbook only if a renter in that situation needs it to act. A fact no situation needs goes on the place's rules page. Taking a statement off a playbook never deletes it.

## Rules every agent follows

- **Stages.** Give each playbook statement one stage, copied exactly from its topic's list below. Never write a new heading. Stages follow the statement order: law, first step, next, if ignored, court.
- **Place named.** Every new statement names its place ("Ohio law says..."). A statement can be shown alone in a search result.
- **Rules pages draft only gaps.** Run `triage gaps <place> <rules-topic>`. Draft one statement per concept marked `gap`, tagged with that concept. A concept answered on another page in the place is refused; the rules page shows it from there.
- **No law found.** When you search and find no law on a gap, say so. The reviewer files a coverage record with `triage nolaw` (place, concept, the official places searched). Never write "{State} has no rule" unless an official source says so in those words; then it is a normal cited statement.
- **National only.** Topics marked national have one page, on united-states. A state's own rules for them go on a rules page.

## Playbooks: situation, what it covers, what goes elsewhere

| Topic | Situation question | Covers | Not here, goes to |
|---|---|---|---|
| cant-pay-rent | "I can't pay my rent. What can I do?" | talking to the landlord, payment plans, paying part of the rent, late fees and grace periods, rent help programs, a landlord refusing rent | any notice to leave or court step: **eviction-defense**; rent still owed after moving out: **move-out-bill** |
| eviction-defense | "I got an eviction notice or court papers. What happens now?" | notices (pay-or-quit included), paying to stop the case, answering, the hearing, default judgment, judgment, time to move, appeal, things left behind, eviction records, free lawyer programs | lockouts and shutoffs to force you out: **locked-out**; a lease that will not be renewed: **lease-renewal** |
| security-deposits | "My landlord didn't return my deposit. What can I do?" | move-out, the return deadline, the itemized list, deductions, roommates, leaving early, what you can sue for | caps, receipts, interest, fees at move-in: the **security-deposit-rules** page; a bill beyond the deposit: **move-out-bill** |
| move-out-bill | "My landlord billed me after I moved out. Do I have to pay?" | damage charges beyond the deposit, rent said to be owed, checking the bill, disputing it in writing, debt collectors, being sued | getting the deposit itself back: **security-deposits** |
| building-sold | "My building was sold or foreclosed. What happens to my lease?" | whether the lease survives, who to pay, the deposit after a sale, foreclosure notice and time to move | the eviction case itself: **eviction-defense** |
| repairs-and-habitability | "My landlord won't fix something. What can I do?" | what must be fixed (mold, pests, lead, alarms included), asking in writing, records, inspections (with the condemnation caution), holding back rent, repair and deduct, rent cuts, moving out (each with its risk) | heat and AC temperatures: **heat-not-working**; leaving because the home is unlivable: **breaking-lease**; utilities off: **utility-shutoff** |
| heat-not-working | "My heat or AC doesn't work. What can I do?" | heat season and temperatures, hot water, AC if the landlord supplied it, your own cooling device, reporting, staying safe in heat or cold | the general duty to repair: **repairs-and-habitability**; utilities shut off: **utility-shutoff** |
| utility-shutoff | "My utilities were shut off. What can I do?" | landlord cut them (link to locked-out), landlord did not pay the bill, I could not pay (winter and medical rules), energy help | the landlord forcing you out: **locked-out** |
| locked-out | "My landlord locked me out. What can I do?" | only a court can remove you, getting back in, getting belongings back, utilities cut to force you out, what a court can order | the court eviction case: **eviction-defense** |
| breaking-lease | "I need to leave before my lease ends. What can I do?" | when the law lets you leave early (abuse, military, unlivable home), what you may still owe, the landlord's duty to look for a new renter, giving notice, the deposit after leaving early | the landlord ending the lease: **lease-renewal** or **eviction-defense** |
| lease-renewal | "My landlord won't renew my lease. What can I do?" | notice to end a month-to-month lease, just cause rules where they exist, answering the notice, when it turns into a court case | the court case itself: **eviction-defense** |
| landlord-entry | "Can my landlord come in?" | notice to enter, allowed reasons, emergencies, saying no, illegal entry, harassment, your own lock rights (rekey, lock change after abuse), quiet enjoyment | lockouts: **locked-out** |
| rent-increase | "My landlord raised my rent. Is that allowed?" | notice periods, caps and rent control, increases during a lease, new fees, utility billing, price gouging in a disaster, retaliatory increases | being told to leave at the end of a lease: **lease-renewal** |
| discrimination | "I think I was treated unfairly because of who I am. What can I do?" | protected groups, what is illegal, housing vouchers, reasonable accommodations, support animals, domestic violence protections, filing a complaint and its deadline, what you can win | retaliation for complaining about repairs: the situation it happened in |
| resource-directory | "Where can I get help?" | one organisation per entry: who it helps, how to reach it | rules of any kind: the topic they belong to |
| constructive-eviction | "My home is so bad I have to leave." | live US and PA pages only, retired once **breaking-lease** publishes there | everything new: **breaking-lease** |

National only (one page, on united-states):

| Topic | Kind | Covers |
|---|---|---|
| move-in-checklist | checklist | photos, the condition report, what to read in the lease, links to deposit rules by place |
| move-out-checklist | checklist | notice, the move-out inspection, photos, forwarding address |
| rental-application | checklist | fees, screening, scams, what to do if turned down; state rules on **rental-application-rules** pages |
| assistance-animal | situation | federal law on service and support animals, asking, if the landlord says no |
| renting-fundamentals | overview | the core rights, each pointing to its situation page |

Cross-cutting concepts (renting-fundamentals: retaliation, records, mediation, small claims, complaint lines, federal housing help) may be tagged on any page where the situation needs them.

## Concepts by area

Tag a statement with the concept whose question it answers. Each topic's rules page asks these questions in this order, with the place added.

### Deposits and moving out

**security-deposits** · stages: What the law says · Ask for your deposit back · If your landlord does not pay · Going to court · rules page: `security-deposit-rules`

- deposit-cap: How much can my landlord charge for a deposit?
- nonrefundable-fees: Can my landlord charge nonrefundable fees (money I will not get back) when I move in?
- holding-deposit: Do I get back a holding deposit (money I paid to hold an apartment)?
- deposit-receipt: Do I get a receipt for my deposit?
- move-in-condition-report: Does my landlord have to give me a condition report (a list of damage already there) when I move in?
- deposit-escrow-interest: Does my deposit earn interest, and where is it kept?
- deposit-increase: Can my landlord raise my deposit?
- deposit-last-month-rent: Can I use my deposit to pay my last month's rent?
- deposit-after-sale: Who gives my deposit back if the building is sold?
- move-out-inspection: Can I ask my landlord to check the home before I move out?
- forwarding-address: Do I have to give my landlord my new address to get my deposit back?
- deposit-return-deadline: How long does my landlord have to return my deposit?
- deduction-itemization: What can my landlord take out of my deposit?
- move-out-charges: Can my landlord bill me for more than my deposit?
- deposit-damages: What can I get if my landlord keeps my deposit unfairly?

**move-out-bill** · stages: What your landlord can charge for · Check the bill · Tell your landlord you disagree · If you are sued or a debt collector calls


**building-sold** · stages: Your lease after a sale · Who to pay rent to · Your deposit · Foreclosure (when the bank takes the building) · rules page: `building-sale-rules`

- foreclosure-tenant-protection: What happens to my lease in a foreclosure (when the bank takes the building from the owner)?

### Can't pay and leaving early

**cant-pay-rent** · stages: What the law says · Talk to your landlord · Get help paying rent · If you still cannot pay · rules page: `rent-payment-rules`

- grace-period: Is there a grace period before rent is late?
- late-fees: How much can my landlord charge for late rent?
- partial-payments: Can I pay part of my rent?
- refused-rent: What if my landlord will not take my rent?
- rent-receipt: Do I get a receipt when I pay rent?
- rent-assistance-programs: Where can I get help paying rent?
- subsidized-rent-change: If the government helps pay my rent, can my part go down when I earn less?
- rent-debt-collection: What happens to rent I still owe?

**breaking-lease** · stages: When you can leave early · What you may still owe · How to leave · After you move out · rules page: `lease-breaking-rules`

- early-termination-rights: When can I end my lease early without paying the rest?
- constructive-eviction: Can I move out because my home is not safe to live in?
- duty-to-mitigate: If I move out early, does my landlord have to look for a new renter?

### Eviction, lockouts and renewal

**eviction-defense** · stages: The notice · Paying to stop the case · Going to court · After the court decides · rules page: `eviction-rules`

- late-rent-notice: How much notice does my landlord have to give when rent is late?
- notice-to-quit: How much notice do I get before my landlord can take me to court?
- pay-and-stay: Can I pay and stop an eviction?
- answer-the-case: What do I do when I get eviction court papers?
- right-to-counsel: Can I get a free lawyer if I am being evicted?
- eviction-court-process: What happens in eviction court?
- default-judgment: What happens if I get a default judgment (I lose because I missed my court date)?
- eviction-appeal: Can I appeal (ask a different court to look at my case again) after I lose an eviction case?
- time-to-move-after-judgment: How long do I have to move out after I lose in eviction court?
- abandoned-property: What happens to things I leave behind when I move out?
- eviction-record: Will an eviction stay on my record, and can I get it removed?

**locked-out** · stages: Only a court can make you leave · Get back into your home · Get your things back · What a court can make your landlord do · rules page: `lockout-rules`

- court-eviction-only: Can my landlord evict me without going to court?
- illegal-lockout: What if my landlord locks me out?

**lease-renewal** · stages: How much notice you get · When your landlord needs a reason · Answer the notice · If your landlord goes to court · rules page: `lease-renewal-rules`

- end-of-tenancy-notice: How much notice does my landlord have to give to end a month-to-month lease?
- just-cause-eviction: Does my landlord need just cause (a reason the law accepts) to make me leave?

### Repairs, heat and utilities

**repairs-and-habitability** · stages: What your landlord has to fix · Ask in writing · Get your home inspected · If it is still not fixed · Moving out · rules page: `repair-rules`

- habitability-standard: What does my landlord have to keep working?
- repair-request-in-writing: How do I ask for repairs?
- mold: Does my landlord have to get rid of mold?
- pests-bed-bugs: Who has to get rid of bed bugs, roaches, or mice?
- lead-paint: Does my landlord have to tell me about lead paint?
- smoke-co-detectors: Does my landlord have to put in smoke and carbon monoxide alarms?
- code-inspection: Who can inspect my home?
- rent-withholding: Can I stop paying rent until repairs are made?
- repair-and-deduct: Can I fix it myself and take it off the rent?
- condemnation-relocation: What help do I get if my home is condemned (the city says no one can live there)?
- casualty-damage: What happens to my lease after a fire or flood?

**heat-not-working** · stages: What the law says · Tell your landlord · Stay safe while you wait · If it is still not fixed · rules page: `heat-and-ac-rules`

- heat-requirement: How warm does my landlord have to keep my home, and do I get hot water?
- ac-requirement: Does my landlord have to fix the air conditioning?
- cooling-device-rights: Can I put in my own air conditioner?

**utility-shutoff** · stages: If your landlord cut them off · If your landlord did not pay the bill · If you could not pay · Help paying the bill · rules page: `utility-shutoff-rules`

- utility-shutoff-protection: Can my utilities (water, electric, gas) be shut off?
- landlord-unpaid-utilities: What if my landlord does not pay the water, gas, or electric bill?
- energy-assistance: Where can I get help paying for heat and electricity?

### Entry and rent

**landlord-entry** · stages: When your landlord can come in · Your locks · If your landlord comes in without notice · rules page: `landlord-entry-rules`

- entry-notice-period: How much notice does my landlord have to give before coming in?
- entry-allowed-reasons: When can my landlord enter?
- emergency-entry: Can my landlord enter in an emergency?
- refuse-entry: Can I say no when my landlord wants to come in?
- entry-penalties: What can I do if my landlord comes in when they should not?
- lock-change-rules: Can I change the locks on my home?
- quiet-enjoyment: Do I have a right to quiet enjoyment (to live in my home in peace)?
- landlord-harassment: What can I do about harassment (my landlord bothering or threatening me)?

**rent-increase** · stages: How much notice you get · Check the new rent · If the increase is not allowed · rules page: `rent-increase-rules`

- increase-notice-period: How much notice do I get before my rent goes up?
- mid-lease-protection: Can my rent go up during my lease?
- rent-control: Is there a limit on rent increases where I live?
- added-fees: Can my landlord add new fees during my lease?
- utility-billing: Can my landlord charge me for water, gas, or electric?
- emergency-price-gouging: Do price gouging laws (rules against raising prices a lot during a disaster) cover my rent?

### Discrimination and applying

**discrimination** · stages: What is against the law · Asking for a reasonable accommodation (a change because of a disability) · File a complaint · What happens after you file · rules page: `discrimination-rules`

- fair-housing: What discrimination is against the law?
- source-of-income: Can a landlord turn me down because I use a housing voucher?
- reasonable-accommodation: Can I ask for a reasonable accommodation (a change in the rules because of a disability)?
- assistance-animal-rules: Can I keep a service or support animal if pets are not allowed?
- domestic-violence-protections: What rights do I have as a renter if I face domestic violence?
- fair-housing-complaint: How do I report a landlord for treating me unfairly?

**rental-application** · rules page: `rental-application-rules`

- application-fees: How much can a landlord charge me to apply?
- tenant-screening: What can a tenant screening report (a background check on renters) show a landlord?
- criminal-history-screening: Can a landlord turn me down because of a criminal record?

**assistance-animal** · stages: What the law says · Ask your landlord · If your landlord says no


### Help and basics

**resource-directory** · stages: Free legal help · Help lines · Help with rent and housing

- help-lines: Who can I call for help?
- legal-aid: Where can I get free legal help?
- housing-counseling: Where can I get housing counseling?

**renting-fundamentals**

- retaliation-protection: Can my landlord punish me for complaining?
- records-and-evidence: What records should I keep?
- mediation: Can a mediator (a neutral person who helps both sides agree) help with my landlord?
- small-claims-court: How do I use small claims court?
- complaint-line: Where can I file a complaint?
- federal-housing-assistance: What federal housing help is there?

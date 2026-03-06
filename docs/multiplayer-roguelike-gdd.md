# Multiplayer Nethack-Style Roguelike: Game Design Document

**Project Codename:** SWARM
**Document Version:** 0.1
**Status:** Design / Pre-Production

---

## Table of Contents

1. [Vision and Scope](#1-vision-and-scope)
2. [Core Nethack Systems](#2-core-nethack-systems)
3. [Multiplayer Architecture](#3-multiplayer-architecture)
4. [Character Creation](#4-character-creation)
5. [World and Dungeon Generation](#5-world-and-dungeon-generation)
6. [Turn System and Synchronization](#6-turn-system-and-synchronization)
7. [Combat](#7-combat)
8. [Inventory and Items](#8-inventory-and-items)
9. [Identification and Discovery](#9-identification-and-discovery)
10. [Monster AI](#10-monster-ai)
11. [Alignment, Gods, and Religion](#11-alignment-gods-and-religion)
12. [Artifacts and Special Items](#12-artifacts-and-special-items)
13. [Multiplayer Modes](#13-multiplayer-modes)
14. [Griefing Mitigation](#14-griefing-mitigation)
15. [Shared Loot and Economy](#15-shared-loot-and-economy)
16. [Player Interaction Systems](#16-player-interaction-systems)
17. [Permadeath in Multiplayer](#17-permadeath-in-multiplayer)
18. [Ghost and Legacy Systems](#18-ghost-and-legacy-systems)
19. [Winning Conditions](#19-winning-conditions)
20. [Feasibility Notes](#20-feasibility-notes)

---

## 1. Vision and Scope

SWARM is a traditional roguelike built on Nethack's mechanical foundations, extended to
support 1-8 players in the same dungeon simultaneously. The design philosophy is:

- **Depth over breadth.** Every system should have emergent interactions.
- **Authentic roguelike feel.** ASCII or tile rendering, turn-based, keyboard-driven.
- **Multiplayer as a first-class concern, not a bolted-on mode.** The dungeon knows
  multiple adventurers are present and responds accordingly.
- **Asymmetric cooperation.** Players are rarely doing the same thing at the same time,
  which is a feature, not a bug.

### Non-Goals (v1)

- Real-time action. Everything is turn-based.
- MMO-scale player counts. Cap is 8 players per dungeon instance.
- Matchmaking or ranked play. Session-based, invite-driven.

---

## 2. Core Nethack Systems

These systems must be preserved, and all multiplayer extensions must be compatible
with their semantics.

### 2.1 Permadeath

Death is permanent and immediate. No respawning. A player's corpse, ghost, and
possessions remain in the dungeon for other players to encounter and loot.

### 2.2 Turn-Based Execution

The dungeon processes in discrete turns. Every entity — player, monster, trap,
timed spell — expends energy and acts when their energy reaches a threshold.
This is the energy-based clock system described in section 6.

### 2.3 Hunger and Resource Attrition

Food consumption, spell point regeneration, and item charges are all time-gated by
the turn counter. Multiplayer speeds the turn count via shared exploration,
so hunger tuning must account for accelerated dungeon traversal.

### 2.4 Identification

Items start unidentified. Players use them, read scrolls, cast spells, or pay sages
to identify them. Identification state is per-player by default (see section 9 for
shared identification rules in multiplayer).

### 2.5 Conduct

Nethack's conduct system (pacifist, vegan, illiterate, etc.) carries over. In
multiplayer, conducts apply per-player. A pacifist player in a party where teammates
kill monsters on their behalf still maintains conduct if they never directly deal
lethal damage.

### 2.6 Bones Files

When a player dies, a bones level is generated. In multiplayer, bones levels are
shared across all sessions on a server instance. A ghost of a dead player can appear
as a uniquely named, difficult monster in another player's future run.

---

## 3. Multiplayer Architecture

### 3.1 Session Model

```
[Server Instance]
  └── [Dungeon Seed + Ruleset]
        ├── Player 1 (authoritative client)
        ├── Player 2
        ├── ...
        └── Player N (max 8)
```

Each dungeon is seeded deterministically. Players join before the run begins or
during the first five dungeon levels (configurable). Late joins place the new player
at the entrance staircase.

### 3.2 Authority Model

The server is authoritative. Clients send intent (move north, attack, pick up item);
the server validates against game state and broadcasts results. There is no peer-to-peer.

This is mandatory for roguelikes: the RNG state must be canonical and shared.

### 3.3 Network Topology

- **Protocol:** WebSocket or TCP over a simple binary frame protocol.
- **State representation:** The dungeon is a shared data structure. Each player sees
  their own fog-of-war overlay on top of it.
- **Delta compression:** Only changed tiles, entity positions, and stat blocks are
  sent per turn, not the full map.
- **Reconnection:** Players who disconnect have their character go into an idle/wait
  state (see 6.5). They can reconnect within a configurable window.

### 3.4 RNG Consistency

A single master RNG stream drives all game events. The server holds this stream.
To keep multiplayer deterministic, all RNG calls must be ordered strictly by
turn resolution order, not by network arrival time.

---

## 4. Character Creation

### 4.1 Races

| Race      | Str | Dex | Con | Int | Wis | Cha | Special                                |
|-----------|-----|-----|-----|-----|-----|-----|----------------------------------------|
| Human     |  0  |  0  |  0  |  0  |  0  |  0  | No resistances; best alignment options |
| Elf       | -1  | +2  | -1  | +1  | +1  |  0  | Sleep resist; infravision; long-lived  |
| Dwarf     | +2  |  0  | +2  | -1  |  0  | -1  | Poison resist; gemology bonus          |
| Gnome     |  0  | +1  |  0  | +2  |  0  |  0  | Infravision; tinker bonus              |
| Orc       | +3  |  0  | +1  | -2  | -1  |  0  | Poison resist; carnivore; feared       |
| Halfling  | -1  | +3  | +1  |  0  |  0  | +2  | Stealth; luck bonus; small             |

Races affect starting equipment weight limits, shopkeeper reactions, and monster
initial hostility thresholds.

### 4.2 Classes (Roles)

All traditional Nethack roles are present. Multiplayer adds party synergy incentives
but no class is locked out of any dungeon branch.

| Class      | Primary Stat | Special Mechanic                                     |
|------------|--------------|------------------------------------------------------|
| Archaeologist | Int       | Detect cursed items; bonus with artifacts            |
| Barbarian  | Str          | Rage mechanic; ignores first 3 points of armor       |
| Caveman    | Str          | Can tame megafauna; starts with pet                  |
| Healer     | Wis          | Shared healing aura (5-tile radius for teammates)    |
| Knight     | Str          | Quest item grants team resistance at adjacency       |
| Monk       | Wis/Dex      | Unarmed synergy; meditation restores alignment       |
| Priest/ess | Wis          | God communication; can bless items for teammates     |
| Ranger     | Dex          | Shares explored fog-of-war with team in LOS          |
| Rogue      | Dex          | Backstab; can pickpocket monsters                    |
| Samurai    | Str/Dex      | Two-weapon mastery; honor mechanics                  |
| Tourist    | Cha          | Credit cards; vendor discounts shared with party     |
| Valkyrie   | Str/Con      | Automatic Mjollnir throw return; Odin prayer bonus   |
| Wizard     | Int          | Spellcasting; can scribe scrolls from memory         |

### 4.3 Alignment

Lawful, Neutral, Chaotic. Each player has independent alignment. Alignment affects:

- Which god they pray to.
- Which artifacts they can wield without penalty.
- How monsters react (cross-aligned priests are hostile).
- Team friction rules (see 13.3).

### 4.4 Party Composition Constraints

No hard constraints. However:

- Having two players with the same class causes both to receive a -1 penalty to their
  class-specific skill caps (gods dislike redundancy among their chosen).
- Having all three alignments represented in a party grants a global +1 luck bonus
  (cosmic balance).
- A full party of the same alignment increases inter-god rivalry events.

---

## 5. World and Dungeon Generation

### 5.1 Dungeon Structure

The dungeon is a vertical structure of levels, procedurally generated from a seed.

```
[Dungeons of Doom] Levels 1-30 (main branch)
  ├── [Gnomish Mines]     Levels 2-5 branch point, 10 levels deep
  │     └── [Minetown]    Fixed special level
  ├── [Sokoban]           Branch off level 7, 4 levels (fixed puzzle)
  ├── [Fort Ludios]       Optional branch, random location
  ├── [Gehennom]          Levels 25-50 (Hell branch, fire-dominated)
  │     └── [Vlad's Tower] Mid-Gehennom branch
  └── [Endgame]           Astral Plane (win condition)
```

### 5.2 Multiplayer Scaling

When N players are in a session, the generator applies scaling factors:

| Players | Monster density | Item count | Trap density | Shop quality |
|---------|----------------|------------|--------------|--------------|
| 1       | 1.0x           | 1.0x       | 1.0x         | Standard     |
| 2       | 1.3x           | 1.4x       | 1.1x         | Standard     |
| 3-4     | 1.6x           | 1.8x       | 1.2x         | Improved     |
| 5-6     | 2.0x           | 2.2x       | 1.4x         | Improved     |
| 7-8     | 2.5x           | 2.6x       | 1.5x         | Enhanced     |

Miniboss rooms (vault guardians, high priests) also scale in difficulty.

### 5.3 Level Persistence

Levels persist fully for the lifetime of the session. Items dropped, walls broken by
wands, altars converted — all state is permanent and shared. If Player A digs a pit
on level 3, Player B falls into it later.

### 5.4 Special Levels

Fixed special levels (Sokoban, Minetown, Medusa's Island) are generated identically
regardless of player count but use the same monster/item density scaling. Sokoban's
puzzles are single-solution; only one player needs to complete them for the party to
claim the prize, but other players can interfere.

---

## 6. Turn System and Synchronization

### 6.1 Energy-Based Clock

All entities use an energy pool:

```
energy_regen = base_speed * speed_modifier
entity acts when energy >= ACTION_THRESHOLD (1000)
```

Standard human speed regenerates 12 energy/tick. A tick fires when the server
processes the next action batch.

### 6.2 Simultaneous Action Resolution

In multiplayer, all players submit their action for the current tick. The server
resolves them in energy order (fastest entity acts first). Within the same energy
tier, resolution is by player join order (consistent across the session).

This means players effectively act in parallel when on different parts of the map,
but on the same tile or in adjacent combat, there is a strict ordering.

### 6.3 Action Submission Window

Players have a configurable window (default: no timeout in co-op; 10 seconds in
competitive) to submit their action. If a player does not submit in time:

- **Co-op mode:** The game waits. No timeout.
- **Competitive/mixed mode:** The player's turn is auto-skipped (the "wait" action).
- **Idle detection:** After 3 consecutive auto-skips, other players can vote to
  convert the idle player to an NPC companion until they reconnect.

### 6.4 Spatial Decoupling

Players on different dungeon levels do not share a turn clock. Level 1 players act
independently of Level 3 players. The server maintains per-level clocks that sync
only when players are on the same level.

This is the most important performance property of the architecture: levels with no
active players can be fast-forwarded or suspended.

### 6.5 Disconnection Handling

On disconnect, a player's character enters a "stunned" state: they do not act but can
be targeted by monsters. After 30 seconds of real time (configurable), teammates can
choose to:

- **Carry the character:** The disconnected player is treated as an inert item that
  can be moved (slowly) by a teammate with sufficient strength.
- **Dismiss the character:** The character is removed from the active dungeon and
  placed at a safe staircase on the current level, consuming one scroll of protection.
  They can rejoin later.

---

## 7. Combat

### 7.1 Melee Combat

The to-hit formula is inherited from Nethack:

```
to_hit = d20 + attack_bonus + weapon_bonus + enchantment
defense = AC_base + armor_bonus + dexterity_modifier
hits if to_hit > defense
```

Damage formula:

```
damage = weapon_dice + strength_modifier + weapon_enchantment
         + class_bonus + conditional_modifiers
```

Conditional modifiers include: silver vs. undead, vorpal edge triggers, elemental
brands, and monster-specific vulnerabilities.

### 7.2 Ranged Combat

Projectiles are discrete entities. A thrown dagger fired across a room can hit an
unintended target — another player, a friendly pet, a shop item.

Friendly fire is on by default in all modes. Players can toggle a "safe throw"
mode that prevents ranged attacks toward tiles occupied by allies, at the cost of
losing the ability to fire through allied squares.

### 7.3 Magic Combat

Spells target tiles, not entities. Area-of-effect spells (fireball, cone of cold)
affect all entities in range including teammates. A Wizard's fireball in a corridor
where a teammate is standing is a player choice with real consequences.

### 7.4 Unarmed and Class Attacks

Monks receive bonus unarmed attacks scaling with experience level. These are
personal and cannot be transferred, but Monks can create training scrolls that
temporarily grant other players one bonus unarmed attack per turn.

### 7.5 Combat Roles and Synergy

Combat synergy is emergent, not prescribed, but some interactions are designed:

- A Rogue flanking a monster attacked by another player gets the backstab bonus.
  (Flanking: the Rogue and at least one other attacker are on opposite sides of
  the monster's tile.)
- A Knight using the challenge action forces a monster to prioritize melee with
  the Knight for 3 turns, allowing ranged teammates to fire safely.
- A Healer's aura tick fires between each monster turn, not between player turns,
  so positioned Healers provide meaningful sustain in extended fights.

### 7.6 Pets and Companions

Each player may have one primary pet. Pets follow their owner. Multiple pets from
different players on the same level are all active. Pets from different players may
fight each other if they are of incompatible species. Owner alignment affects pet
loyalty decay in cross-alignment parties.

---

## 8. Inventory and Items

### 8.1 Inventory Architecture

Inventory is strictly per-player. There is no shared chest or party inventory.
Trading requires explicit drop/pick-up actions or the new direct-trade gesture.

### 8.2 Direct Trade

Two adjacent players can open a trade screen. Each party proposes items to exchange.
Both must confirm. The exchange is atomic — either both sides transfer or neither does.
Dropping items and having someone else pick them up remains valid as informal trade.

### 8.3 Weight and Encumbrance

Each player has independent carrying capacity based on Strength. Carrying encumbered
incurs speed penalties. Carrying burdened halves speed. Over-burdened prevents movement.

A high-Strength character can carry items for another player by explicitly picking
up items they don't intend to use personally, but this comes at the encumbrance cost
to the carrier.

### 8.4 Containers

Bags, chests, and boxes are in the world, not in inventory (with the exception of
bags of holding, which are inventory items). A chest on level 4 is accessible to any
player who reaches it. First player to open it gets first pick; contents persist for
others.

### 8.5 Bags of Holding

Only one bag of holding can exist per dungeon run (rarity maintained from Nethack).
Two bags of holding do not coexist; the second one found is automatically identified
as a bag of tricks. This prevents item hoarding via multiple bags.

### 8.6 Item Cursing and Blessing

Blessed items provide bonuses; cursed items may be stuck to the wearer. A Priest
can bless or uncurse items for teammates within 2 tiles by expending a prayer charge.
This does not require altar proximity, but altar blessings are stronger (+1 extra
enchantment level on blessed items).

### 8.7 Scrolls, Potions, Wands

- **Scrolls:** Single-use, consumed on reading. Effect is immediate.
- **Potions:** Single-use, consumed on drinking. Can be thrown (splash damage/effect).
- **Wands:** N charges, recharge once safely. Wands of wishing: one per dungeon,
  and wishing is broadcast to all players ("You hear a wish being granted...").

---

## 9. Identification and Discovery

### 9.1 Per-Player Identification

Each player maintains their own identification state. If Player A has identified
that the blue potion is a potion of healing, Player B sees only "blue potion" until
they identify it themselves.

### 9.2 Knowledge Sharing

Players can share identification knowledge through:

- **Verbal communication:** Players type in chat "blue potion = healing." This is
  out-of-band and has no mechanical effect — other players still see "blue potion"
  until they identify it, but they now know what to expect.
- **Identify scroll (shared cast):** A Wizard with the Identify spell can cast it
  on a teammate's item, revealing its identity to both the Wizard and the item's owner.
- **Sage NPC:** Paying a sage identifies the item for the payer. The payer can then
  share via chat.

### 9.3 Class-Based Identification

- Archaeologists passively identify all artifacts on sight.
- Healers identify all potions after drinking one of each color.
- Wizards identify scrolls after reading one of each glyph.

### 9.4 Collaborative Identification

When the same unknown item type is used by two different players in the same session
and both receive the same result, the game flags that item type as "strongly suspected"
and shows a confidence indicator. This is informational only — no automatic
identification occurs.

---

## 10. Monster AI

### 10.1 Threat Assessment

Monsters now have multi-player threat assessment. They evaluate the nearest player,
the most dangerous player (highest visible weapon enchantment or recent damage
output), and the most vulnerable player (lowest HP, lowest AC).

Each monster species has a weight vector over these three targets:

- **Mindless (zombie, skeleton):** nearest only.
- **Pack hunters (wolves, gnolls):** split between nearest and most vulnerable.
- **Intelligent (mind flayers, liches):** most dangerous first, then most vulnerable.
- **Cowardly (nymphs, leprechauns):** move away from all players; target items.

### 10.2 Aggro and Memory

Monsters have aggro state: neutral, alert, hostile. In multiplayer:

- Attacking a monster makes it hostile to the attacker specifically.
- A monster can have split aggro: hostile to Player A (who hit it), alert to
  Player B (who it can see), neutral to Player C (out of range).
- A monster that kills a player gains a brief frenzy buff (+2 to-hit, +1 damage for
  5 turns) and becomes hostile to all remaining visible players.

### 10.3 Coordination

Some intelligent monsters coordinate:

- **Soldiers/guards:** Share aggro state within 10 tiles. If one is attacked,
  others in range enter alert state.
- **Demons in Gehennom:** Can summon reinforcements targeting the player with the
  highest experience level.
- **Riders (Death, War, Famine, Pestilence):** When multiple Riders are active
  simultaneously on the same level (possible in large parties), they act in concert,
  targeting different players simultaneously.

### 10.4 Reaction to Multiple Players

Monsters that might flee from one player will not flee from a party. Cowardly
monsters, when cornered by multiple players, receive a desperation buff and become
temporarily aggressive.

### 10.5 Leader-Follower Dynamics

Some monsters have follower hierarchies. Killing the leader causes followers to
scatter (10% chance each to flee level) or frenzy (90% chance). The leader kill
credit goes to the player who dealt the killing blow, affecting that player's kill
count for class rank advancement.

---

## 11. Alignment, Gods, and Religion

### 11.1 The Three Pantheons

Alignment is a sliding scale from -100 (deeply chaotic) to +100 (deeply lawful),
with neutral centered at 0. Each player tracks this independently.

**Lawful Gods:** Tyr, Mitra, Aten
**Neutral Gods:** Lugh, Hermes, Isis
**Chaotic Gods:** Loki, Mog, Kos

Each character is assigned a god on creation based on class and alignment.

### 11.2 Prayer Mechanics

Prayer consumes the "god favor" resource. Favor accumulates through:

- Killing cross-aligned monsters.
- Offering valuable items on altars.
- Completing conduct milestones.
- Reaching class-specific achievement thresholds.

Favor is depleted by:

- Praying too frequently (diminishing returns, cooldown).
- Performing acts contrary to alignment.
- Being killed by a minion of one's own god (shame penalty).

### 11.3 Multiplayer Prayer Dynamics

The same altar can be used by multiple players. However:

- **Cross-alignment altar use:** If a lawful altar is converted by a chaotic player,
  it becomes a chaotic altar. All lawful players on that level receive an alignment
  penalty and their god's anger grows.
- **Contested altars:** Two players of different alignments cannot simultaneously
  use the same altar. The second to arrive must wait or fight for it (gods watch).
- **Sacrificial priority:** If two players sacrifice on the same altar in the same
  turn, the one with higher current favor receives full credit; the other receives
  half credit.

### 11.4 God Interactions Across Players

Gods are aware of all players in the dungeon. This creates:

- **Aligned bonus:** If two lawful players both have high favor with the same god,
  that god occasionally grants both players a passive blessing simultaneously.
- **Cross-alignment tension:** A chaotic player repeatedly converting altars may
  cause the lawful god to temporarily withdraw from a lawful teammate ("Your god
  is angered by the corruption in this place").
- **Rival god interference:** If a chaotic player summons a demon via chaotic prayer,
  that demon is hostile to all lawful/neutral players in the dungeon, not just the
  summoner.

### 11.5 High Priest Encounters

Each alignment has a high priest in the dungeon's depths. Killing the high priest
of a player's own alignment is a massive alignment penalty (usually game-over for
that player's standing). Killing a rival alignment's high priest is a major gift.

In multiplayer, a chaotic player killing a lawful high priest while a lawful player
is in the dungeon creates a confrontation event: the lawful player's god demands
vengeance, making the chaotic player a marked target for divine punishment.

---

## 12. Artifacts and Special Items

### 12.1 Artifact Uniqueness

Each dungeon instance (not per-player) contains exactly one of each artifact. This
is a core tension source in multiplayer.

Key artifacts and their multiplayer behaviors:

| Artifact         | Effect                                   | Multiplayer Note                              |
|------------------|------------------------------------------|-----------------------------------------------|
| Excalibur        | +5 lawful holy blade                     | Usable only by lawful characters; chaotic     |
|                  |                                          | wielder takes 1d20 damage per turn            |
| Mjollnir         | Returning thrown hammer, lightning       | Returns to thrower's tile, not to hand;       |
|                  |                                          | another player can intercept the return       |
| The Amulet of    | Win condition item; slows hunger;        | Only one can win with it; others must         |
| Yendor           | detects nearby players                   | be on Astral Plane simultaneously             |
| Vorpal Blade     | Instant kill on natural 20               | Confirms target is dead; no splitting kills   |
| The Bell of      | Required for endgame sequence            | Any player can ring it; sequence is           |
| Opening          |                                          | shared progress                               |
| The Candelabrum  | Required for endgame sequence            | Same: shared progress item                    |
| The Book of the  | Required for endgame sequence; readable  | Reading it by wrong alignment causes         |
| Dead             | only by Wizard of correct alignment      | dungeon-wide monsters-go-berserk event        |

### 12.2 Artifact Conflict

A player cannot wield an artifact that conflicts with their alignment without taking
damage. However, they can carry it (at risk). Another player can demand the artifact
be surrendered (a formal game mechanic: the demand is logged and the other player
must accept or refuse within 5 turns).

### 12.3 Crowning and Wishing

Crowning (a god granting an artifact as a gift) is per-player. A player who is
crowned receives a unique class artifact. If two players of the same class are in
the party, the second to reach crowning threshold receives the artifact as a lesser
version (-1 enchantment).

Wishing for an artifact that another player already carries: the wish is granted but
the item appears with a "disputed" flag. The original carrier's god is notified
(flavor text) and the god's favor with the wisher decreases slightly.

---

## 13. Multiplayer Modes

### 13.1 Full Co-op Mode

All players work toward the same goal: at least one player must reach the Astral Plane
and offer the Amulet of Yendor. Victory condition: the offering is made. All living
players on the Astral Plane at the time of offering are credited with a win.

**Key rules:**
- No player-versus-player combat. Attacking a teammate does nothing.
- Shared map memory (all players see each other's explored tiles).
- Death is still permanent for the dead player, but surviving players continue.
- A run succeeds even if only one player survives to the end.

### 13.2 Competitive Mode (Race)

All players are racing to win individually. First to offer the Amulet wins.

**Key rules:**
- Full friendly fire enabled.
- Players can steal items from each other's inventories by attempting pickpocket
  (Rogue ability) or by attacking and looting the corpse.
- Fog-of-war is fully per-player; no shared exploration.
- Players can see each other on the same level (you can see other adventurers).
- Killing another player grants XP equal to their level * 100.

### 13.3 Mixed Mode (Factions)

Players are divided into 2-4 factions (alignment-based is the default split, but
custom factions are supported). Factions cooperate within themselves; compete across.

**Key rules:**
- Within a faction: co-op rules apply.
- Across factions: competitive rules apply.
- Factions share fog-of-war within the faction.
- First faction to have a member offer the Amulet wins; all living members of that
  faction are credited.

Faction assignment at session start. Cross-faction communication is possible via
a "diplomatic channel" but it is a separate chat channel from the faction channel.

### 13.4 Betrayal Mode (Hidden Traitor)

An extension of co-op mode: one player (the Traitor) is secretly assigned a
competitive win condition. The Traitor must eliminate all other players (directly
or indirectly) and then win solo, or ensure the offering is never made.

Traitor mechanics:
- The Traitor can attack other players (but it looks like a monster attack to the
  victim's UI: "Something unseen strikes you!" unless they use a ring of see
  invisible or are polymorphed to see alignments).
- The Traitor has a hidden chaotic alignment that grows as they betray allies.
- At deep chaotic alignment, the Traitor begins to glow faintly (visible to others).
- Traitor win: all other players dead, Amulet offered.
- Cooperative win: Traitor is identified and killed before the win condition.

---

## 14. Griefing Mitigation

### 14.1 In Co-op Mode

Since direct PvP is disabled in co-op, griefing takes environmental forms:

- **Altar conversion:** Prevented by rule — players cannot convert altars to their
  alignment if another player is within 5 tiles and would be negatively affected.
- **Staircase blocking:** A player cannot remain idle on a staircase for more than
  10 turns while another player is on the same tile attempting to use it.
- **Resource denial (item hoarding):** No game-enforced limits, but the karma system
  (below) discourages it.
- **Wand of death misfires:** In co-op, wand/spell area attacks are logged; if a
  player triggers more than 3 ally deaths from wand/spell misfire in a run, they
  receive the "reckless" flag and other players can vote to boot them.

### 14.2 Karma System

Every player has a karma score per session. High karma unlocks minor cosmetic effects
and a bonus at the end-of-run scorecard. Karma decreases for:

- Killing allied pets.
- Triggering traps that damage allies.
- Taking items from a corpse of an ally within 5 turns of that ally's death
  (grave-robbing timer).
- Blocking an ally from a staircase.

### 14.3 Vote-Kick

In any mode, all remaining players (minus the accused) can call a vote to remove a
player from the session. Requires unanimous agreement. The removed player's character
is placed in the nearest safe room and enters a permanently idle state for the rest
of that dungeon instance. Their items and gold remain accessible.

### 14.4 Session Contracts

At session start, the host configures a session contract:

- **Friendly fire:** on/off per-mode default.
- **Item lock:** Items in a player's inventory for more than 5 levels cannot be
  stolen (prevents regret-stealing of long-held items).
- **Altar respect:** Cross-alignment altar conversion blocked or allowed.
- **Communication:** Voice, text, or silent (no chat at all, for challenge runs).

---

## 15. Shared Loot and Economy

### 15.1 Loot Distribution

Items spawned in the world are unclaimed. First player to pick them up owns them.
Items on a monster's corpse are unclaimed for 10 turns after death; after that they
belong to whoever picks them up first.

### 15.2 Kill Credit and XP

Experience points from a kill go to the player who dealt the killing blow. Assisting
in a kill (dealing at least 25% of the monster's max HP in damage) grants 30% of the
kill's XP to each assistant (the killer still gets 100%).

This can result in a player receiving 130% of a monster's XP value if they killed
it with three assisting teammates — but the assisting teammates each only get 30%.
This incentivizes roles: tank to hold aggro, damage dealers to assist.

### 15.3 Gold

Gold is per-player. Each player has their own gold pile. Monsters drop gold that
is unclaimed until picked up. Shops are per-player transactions.

### 15.4 Shops in Multiplayer

Shopkeepers track per-player debts. If Player A breaks a shop item and doesn't pay,
the shopkeeper is hostile to Player A only. Other players can still shop normally —
unless they attempt to pay Player A's debt on their behalf (which is possible, as a
diplomatic gesture).

### 15.5 Selling Items to Shops

Players can sell items to shops. A sold item becomes shop inventory and any player
can buy it. This creates an informal economy: a player who finds a dagger+5 early
can sell it; a later player can buy it.

Shop sell prices are standard (50% of buy price). There is no player-to-player
haggling mechanic beyond the direct trade gesture (section 8.2).

---

## 16. Player Interaction Systems

### 16.1 Communication

- **Global chat:** All players on same dungeon instance see it.
- **Level chat:** Only players on same level.
- **Whisper:** Targeted message to one player.
- **Emote system:** Nethack-flavored emotes (e.g., /bow, /taunt, /pray) that appear
  as dungeon message log entries ("Aldric the Wizard bows his head in reverence").

### 16.2 Gestures (Game Mechanics)

Adjacent players can perform mechanical gestures:

| Gesture        | Effect                                                        |
|----------------|---------------------------------------------------------------|
| Offer trade    | Opens trade screen (section 8.2)                             |
| Bless/curse    | Priest blesses/uncurses one item in adjacent player's inv     |
| Pickpocket     | Rogue steals one random item (competitive/betrayal only)      |
| Assist         | Transfer one potion of healing to adjacent player             |
| Drag           | Move an incapacitated/disconnected player one tile per turn   |
| Share memory   | Transfer your identified item knowledge to adjacent player    |
| Challenge      | (Knight) Forces a monster to prioritize you for 3 turns       |

### 16.3 Telepathy and Awareness

Players see other players as named glyphs on their map (@ symbol with a color code
per player). On the same level, they can always see each other's position — there is
no stealth between players in co-op mode.

In competitive mode, players can be stealthy: a Rogue with a ring of stealth is
not shown on other players' minimaps unless they are in line-of-sight.

### 16.4 Player Notes

Any player can annotate a dungeon level with notes visible to all teammates. Notes
persist for the level. Examples: "Teleport trap at (12,5)," "Boss room west of here."
This is in-world as chalk marks or carved text.

---

## 17. Permadeath in Multiplayer

### 17.1 Individual Death

When a player dies, they are permanently removed from active play for that session.
Their corpse, ghost, and possessions remain on their death tile. Other players may
loot the corpse after the 10-turn grave-robbing protection expires.

### 17.2 Death Messages

Death is broadcast to all players: "Arnulfr the Valkyrie was killed by a minotaur
on level 14." This is shown in the dungeon message log in red, visible regardless
of what level the player is on.

### 17.3 Ghost Companions

A dead player may optionally have their ghost remain as an NPC companion for the
surviving party. The ghost:

- Can communicate (text only, shown as pale blue messages).
- Can observe but not interact with the world.
- Can warn surviving players of dangers they remember from their life.
- Has limited ability to frighten monsters (1/day, fear effect on monsters in 3-tile
  radius, no damage).

The ghost dissipates when the session ends (win or all-dead).

### 17.4 Revival (Optional Rule)

The host may enable a revival mechanic. Revivals are extremely expensive:

- A scroll of resurrection exists in the dungeon (1 per instance, extremely rare,
  found in Gehennom or as a god gift).
- Reading it while standing on an ally's grave raises them at 1 HP with no items
  (all items remain on the grave tile).
- Revived players retain their level but lose all alignment bonuses and start with
  neutral alignment.

This is off by default as it dilutes permadeath tension.

### 17.5 Spectator Mode

Dead players can spectate any surviving player's view in real time. They see
exactly what that player sees: their fog-of-war, their inventory, their stats.
They cannot communicate in-game (to prevent spoiling to the living) but can use
a separate out-of-band spectator chat visible only to other dead players and any
external observers.

---

## 18. Ghost and Legacy Systems

### 18.1 Bones Levels (Server-Wide)

When any player dies, a bones level entry is created for that dungeon level. In the
next session that uses that dungeon level (different seed, different players), there
is a 5% chance the level is a bones level. The bones level contains:

- The dead player's ghost (uniquely named, uses player's stats at death).
- The player's equipment and gold scattered near the ghost.
- Additional monsters attracted to the ghost's presence.

### 18.2 Memorial Engravings

When a player dies, a memorial engraving is automatically written on the floor of
the death tile: "Here died [Name] the [Title], slain by [monster] on turn [N]."
This engraving persists in bones levels.

### 18.3 Hall of Records

The server maintains a persistent Hall of Records across all sessions. Records
tracked: highest score, deepest level reached, most monsters killed, most items
identified, fastest win, highest-level death. Top entries include whether the run
was solo or multiplayer and the party composition.

---

## 19. Winning Conditions

### 19.1 Standard Win

Obtain the Amulet of Yendor (located on level ~45) and ascend to the Astral Plane.
On the Astral Plane, the player must find the correct altar (one of three, alignment-
specific) and offer the Amulet.

In multiplayer:

- Only one player can carry the Amulet (it prevents teleportation and slows the
  carrier, creating an interesting "bearer" role).
- All living players who reach the Astral Plane simultaneously with the Amulet
  bearer are credited with the win.
- Players can escort the bearer or race ahead to scout.

### 19.2 The Three Altars of the Astral Plane

The Astral Plane has three altars (lawful, neutral, chaotic) guarded by Angels and
Riders. In multiplayer, all Riders are active simultaneously, one per altar.

The correct altar for winning is the bearer's alignment. If the bearer has cross-
aligned themselves (via altar conversion or chaotic acts), they must find the altar
matching their current alignment.

Multiple-alignment parties may find this challenging if the bearer changed alignment
mid-dungeon — they may need a Priest to realign them before offering.

### 19.3 Alternative Win Conditions (Optional)

These can be toggled on by the host:

- **Conquest:** Slay all four Riders, Death, War, Famine, Pestilence. Requires
  splitting the party across the Astral Plane.
- **Apotheosis:** All players reach experience level 30 and offer themselves (not
  the Amulet) to their respective gods. The Amulet remains uncontested. Victory
  is philosophical, not material.
- **Extraction:** Escape the dungeon with the Amulet of Yendor by the up-staircase
  on level 1 (rather than the Astral Plane route). Harder in some ways (more
  dungeon floors to re-traverse) but bypasses the Astral Plane.

---

## 20. Feasibility Notes

This section is intended for engineering review.

### 20.1 Authoritative Server Architecture

The central challenge is that Nethack's original code is deeply single-player
and assumes one agent interacting with one dungeon. A multiplayer server cannot
use NetHack's source directly; the game logic must be re-implemented with:

1. **Stateless action handler:** Each action is a pure function over game state.
2. **Event sourcing:** All state changes are events; the game state is the
   accumulation of events. This enables reconnection, spectating, and replay.
3. **Per-entity energy scheduling:** The turn clock must be a priority queue over
   all entities (players + monsters + traps), not a loop with implicit ordering.

### 20.2 RNG Coherence

A single RNG stream means actions must be serialized. Consider a two-level RNG:

- **World RNG:** Seeded at dungeon creation; used for level generation only.
- **Action RNG:** Seeded from world seed + turn counter; used for all combat/
  item/event resolution. This allows independent level generation while keeping
  action RNG deterministic given the action history.

### 20.3 Scalability

With 8 players and the scaling rules in section 5.2, a dungeon may have ~2.5x normal
monster counts. Each monster requires AI evaluation each turn. At level 30 of
Gehennom, this could be 100-200 active monsters on one level. Standard AI evaluations
are O(1) per monster; path-finding is O(N log N) in map size. This is manageable
on a single server thread for maps of 80x24 (standard Nethack size).

For larger maps, spatial partitioning (quadtree over active entities) is recommended.

### 20.4 State Synchronization Size

At each turn, the delta sent to each client includes:
- Changed tile list (typically 0-20 tiles per turn per player action).
- Entity position updates (moved monsters, projectiles).
- Status effect changes.
- Inventory changes for that player.
- Message log entries.

This is well within the bandwidth of a websocket connection even at 100ms tick rates.
For ASCII rendering clients, tile changes are just character + color pairs (2 bytes each).

### 20.5 Identified Areas of Design Risk

1. **Simultaneous altar use:** Edge cases in cross-alignment altar conflicts when
   two players act in the same tick. Resolution order must be strictly defined.

2. **Artifact uniqueness enforcement:** The "one artifact per dungeon" invariant
   must be enforced at the state level, not just at generation time. Wishing,
   bones level imports, and artifact duplication glitches must all be guarded.

3. **Turn timer in co-op:** No-timeout co-op means one AFK player blocks the
   session. The idle-to-NPC conversion (section 6.3) is the safety valve but
   must be carefully tested for edge cases (player in combat, on a trap tile, etc.).

4. **Betrayal mode information leakage:** The Traitor identity must not be
   inferrable from network traffic patterns, server-side logs visible to players,
   or UI rendering differences. The Traitor's "invisible attack" visual must be
   indistinguishable from a standard invisible monster attack at the client level.

5. **Bones level cross-session balance:** A very powerful player who dies leaves
   a very powerful ghost. This can create unfair difficulty spikes in sessions
   that encounter that bones level. A difficulty cap on imported bones ghosts
   (scaled to the receiving session's median player level) is recommended.

### 20.6 Technology Recommendations

- **Server language:** Rust or Go for the game logic server (performance, memory
  safety, concurrency primitives align well with the authoritative server model).
- **Client:** Terminal client (curses/ncurses) for authenticity; optional tile
  client using SDL2 or a web canvas renderer for accessibility.
- **Serialization:** MessagePack or CBOR for compact binary game state deltas.
- **Persistence:** SQLite for single-server deployments; PostgreSQL for multi-server
  with shared bones file pool.

### 20.7 Open Design Questions

These questions should be resolved before engineering begins:

1. Should alignment change be visible to other players in real time, or only at
   death? Visible change creates information and potential betrayal detection.
   Hidden change creates more dramatic reveals.

2. Should the Amulet of Yendor be transferable between players? If yes, who "wins"
   — the final bearer or the player who first held it? If no, the game reduces to
   "whoever finds the Amulet first must be protected."

3. In competitive mode, should dead players' ghosts be hostile NPCs (as in Nethack
   bones), friendly spectators, or player-controlled? Player-controlled ghosts in
   competitive mode could be a powerful and interesting mechanic (haunt your killer).

4. How should the Sokoban puzzle work in multiplayer? Allow parallel solutions per
   player (each sees their own version of Sokoban, only one instance of the prize),
   or one shared puzzle that requires cooperation?

5. Should the hunger clock be shared (all players eat at the same dungeon turn rate)
   or per-player (based on individual action count)? Per-player is more fair but
   harder to communicate; shared is simpler but penalizes slow players.

---

*End of SWARM Game Design Document v0.1*

*This document is intended as a creative and technical foundation for review.*
*All systems described are proposals subject to revision based on feasibility*
*and playtesting feedback.*

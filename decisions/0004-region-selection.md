# ADR-0004: Region Selection

## Status

Accepted

## Context

We need to select a GCP region for the Claude Code sandbox VM. Requirements:

- European location (for latency and data residency)
- Support for C4A (Axion ARM) machine types
- Cost-effective pricing tier
- Good availability

GCP has 13 European regions with varying pricing tiers:

- **Tier 1** (cheapest): Belgium, Netherlands, Finland, Stockholm
- **Tier 2**: Frankfurt, Zurich, London, Warsaw, Milan, Madrid, Paris, Turin, Berlin

## Decision

Use **Stockholm (europe-north2)** as the primary region.

Zone: `europe-north2-a`

## Options Considered

### Option 1: Belgium (europe-west1)

Most established European region.

**Pros:**

- Tier 1 pricing
- Mature region with full service availability
- Central European location

**Cons:**

- High demand, potentially more spot preemptions
- Slightly higher latency to Nordic locations

### Option 2: Finland (europe-north1)

Nordic region in Hamina.

**Pros:**

- Tier 1 pricing
- C4A availability
- Cooler climate (efficient cooling)
- Low carbon energy

**Cons:**

- Higher latency to central/southern Europe
- Smaller region than Belgium

### Option 3: Stockholm (europe-north2) (Chosen)

Nordic region in Stockholm.

**Pros:**

- Tier 1 pricing (cheapest tier)
- C4A (Axion ARM) availability
- Nordic location with good connectivity
- Newer region, potentially less contention
- Low carbon energy (Sweden's grid)

**Cons:**

- Newer region (2022), slightly fewer services than Belgium
- Higher latency to southern Europe

### Option 4: Frankfurt (europe-west3)

Central European hub.

**Pros:**

- Central location, good connectivity
- Full service availability
- Major internet exchange point

**Cons:**

- Tier 2 pricing (~10-15% more expensive)
- High demand region

## Consequences

### Positive

- Tier 1 pricing: lowest compute costs in Europe
- C4A availability: can use cost-effective ARM instances
- Good connectivity to UK/Europe for remote development
- Sustainable: Sweden's electricity grid is largely renewable

### Negative

- Higher latency to southern European locations
- Fewer zones than larger regions (less redundancy options)

### Neutral

- Zone `europe-north2-a` selected as primary
- Can migrate to other Tier 1 regions if availability issues arise
- Stockholm gaining more services over time as region matures

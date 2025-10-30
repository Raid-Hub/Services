# Queue Worker Autoscaling Implementation Audit

## Overview
This document outlines issues and improvements needed for the queue worker autoscaling system in Hermes (queue workers).

## Executive Summary

The queue worker autoscaling system **works but has accuracy issues** due to:

1. **Oversimplified Scaling Logic** (Critical)
   - Only considers queue depth, not processing rate or throughput
   - No tracking of actual messages processed per worker
   - Ignores message processing time differences

2. **Reactive, Not Predictive** (Critical)
   - Waits for queue to fill before scaling up
   - No consideration of processing velocity
   - May lag behind actual demand

3. **Scaling Granularity Issues** (Medium)
   - Percentage-based scaling can be too aggressive or too conservative
   - No dead zone/hysteresis causing oscillation
   - Fixed check interval (5 minutes) may be too slow for bursty workloads

4. **No Feedback Loop** (Medium)
   - Doesn't verify if scaling decisions were effective
   - No learning from past scaling decisions
   - No tracking of scaling effectiveness

**Priority Fixes**: Add throughput tracking, implement dead zone, consider processing rate, add feedback loop.

## Current Implementation

### Core Components
1. **TopicManager** (`lib/messaging/processing/topic_manager.go`): Manages workers and scaling
2. **Worker** (`lib/messaging/processing/worker.go`): Processes individual messages
3. **Topic Config** (`lib/messaging/processing/topic.go`): Defines scaling parameters per topic
4. **Queue Workers** (`queue-workers/*.go`): Topic-specific configurations

### Scaling Algorithm
- Checks queue depth every 5 minutes (default)
- If `queueDepth > ScaleUpThreshold` AND `workers < MaxWorkers`: Scale up by `ScaleUpPercent`
- If `queueDepth < ScaleDownThreshold` AND `workers > MinWorkers`: Scale down by `ScaleDownPercent`
- Otherwise: No change

## Critical Issues

### 1. **Queue Depth Only Metric**
**Issue**: Scaling decisions are based solely on queue depth (`q.Messages`), ignoring:
- Processing rate (messages/second)
- Processing time per message
- Worker utilization
- Message arrival rate

**Location**: `topic_manager.go:287-327`, `topic_manager.go:330-350`

**Impact**: 
- System may scale up unnecessarily when queue is large but processing is fast
- System may not scale up quickly enough when processing is slow
- Different message types have different processing times (not accounted for)

**Example**: 
- InstanceStore: Large queue (1000) but fast processing → scales up unnecessarily
- PlayerCrawl: Small queue (50) but slow processing → doesn't scale up fast enough

**Recommendation**:
- Track messages processed per worker per second
- Track average processing time per message
- Calculate throughput: `messagesPerSecond = queueDepth / timeToClear`
- Scale based on throughput delta, not just queue depth

### 2. **No Processing Rate Tracking**
**Issue**: No metrics collected on:
- How many messages each worker processes per second
- Average processing time per message type
- Worker utilization (idle vs busy time)

**Location**: `worker.go:38-89`, `topic_manager.go:287-327`

**Impact**: Cannot make informed scaling decisions without knowing actual processing capacity

**Recommendation**:
- Add Prometheus metrics for messages processed per worker
- Track processing time distribution
- Calculate worker throughput: `messagesProcessed / workerTime`
- Use metrics to predict how many workers needed to clear queue

### 3. **Fixed Check Interval**
**Issue**: Scaling checks happen every 5 minutes regardless of:
- Queue growth rate
- Current queue depth
- Processing urgency

**Location**: `topic_manager.go:289`, `topic.go:20`

**Impact**: 
- May take 5 minutes to react to sudden queue growth
- Bursty workloads may overwhelm system before scaling kicks in
- Slow reaction time for urgent queues

**Recommendation**:
- Dynamic check interval based on queue depth
- Faster checks when queue is growing rapidly
- Consider exponential backoff on check frequency when stable

### 4. **Percentage-Based Scaling Issues**
**Issue**: Scaling by percentage has problems:
- Small worker counts: `20% of 5 workers = 1 worker` (too small)
- Large worker counts: `20% of 50 workers = 10 workers` (too large)
- No consideration of actual need (e.g., need 2 workers but 20% of 10 = 2, okay; but need 2 workers and 20% of 50 = 10, wasteful)

**Location**: `topic_manager.go:308-315`

**Impact**: 
- Overscaling when worker count is high
- Underscaling when worker count is low
- Inefficient resource usage

**Recommendation**:
- Scale based on target throughput: `workersNeeded = targetThroughput / currentWorkerThroughput`
- Use percentage as fallback only
- Add minimum/maximum step sizes (e.g., always scale by at least 2, at most 10)
- Consider: `workersNeeded = ceil(queueDepth / messagesPerWorkerPerMinute)`

### 5. **No Dead Zone/Hysteresis**
**Issue**: Scaling decisions happen every check interval if thresholds are crossed:
- If queue depth fluctuates around threshold, workers oscillate
- No stability period to prevent rapid scaling up/down
- No consideration of recent scaling decisions

**Location**: `topic_manager.go:287-327`

**Impact**: Worker count oscillation, wasted resources, unstable system

**Recommendation**:
- Add dead zone: only scale if change persists for N checks
- Add hysteresis: different thresholds for scale-up vs scale-down (e.g., scale up at 1000, scale down at 500)
- Track recent scaling decisions: don't scale again within X minutes
- Add cooldown period after scaling

### 6. **No Feedback Loop**
**Issue**: Scaling decisions don't verify effectiveness:
- Doesn't check if scaling up actually reduced queue depth
- Doesn't check if scaling down caused queue to grow
- No learning from past decisions

**Location**: `topic_manager.go:287-327`

**Impact**: System may make ineffective scaling decisions repeatedly

**Recommendation**:
- Track queue depth before and after scaling
- Compare expected vs actual queue reduction
- Log scaling effectiveness metrics
- Adjust thresholds based on effectiveness

### 7. **Doesn't Account for Message Processing Time**
**Issue**: Different message types have vastly different processing times:
- InstanceStore: Fast (database writes)
- PlayerCrawl: Slow (API calls, complex processing)
- CharacterFill: Medium (mixed operations)

**Location**: All queue worker files, `topic_manager.go:287-327`

**Impact**: Same scaling logic applied to all topics regardless of processing complexity

**Recommendation**:
- Track per-topic processing time averages
- Adjust scaling thresholds based on processing time
- Consider: `workersNeeded = queueDepth * avgProcessingTime / targetLatency`

### 8. **Ignores Queue Growth Rate**
**Issue**: Only looks at current queue depth, not:
- How fast queue is growing
- Arrival rate vs processing rate
- Trend analysis

**Location**: `topic_manager.go:294`, `topic_manager.go:330-350`

**Impact**: May scale up after queue already filled, too late

**Recommendation**:
- Track queue depth over time
- Calculate queue growth rate: `(currentDepth - previousDepth) / timeDelta`
- Scale proactively when growth rate > processing rate
- Use exponential weighted moving average for trends

### 9. **Scale Down Too Aggressive**
**Issue**: When queue depth drops below threshold, scales down immediately:
- May scale down while messages are still arriving
- No consideration of steady-state vs temporary dip
- May cause ping-pong effect

**Location**: `topic_manager.go:312-315`

**Impact**: Rapid scale-down then scale-up cycles

**Recommendation**:
- Scale down more conservatively (lower percentage or longer wait)
- Require queue depth to stay below threshold for multiple checks
- Consider arrival rate: don't scale down if messages arriving faster than processing

### 10. **No Consideration of Resource Constraints**
**Issue**: Scaling decisions don't consider:
- System resource usage (CPU, memory, connection limits)
- Database connection pool limits
- API rate limits
- Network bandwidth

**Location**: `topic_manager.go:287-327`

**Impact**: May scale beyond system capacity, causing degradation

**Recommendation**:
- Integrate with monitoring to check resource usage
- Don't scale up if resources are constrained
- Consider per-topic resource limits

## Medium Priority Issues

### 11. **Error Handling on Queue Depth Check**
**Issue**: If `getQueueDepth()` fails, scaling is skipped for that interval:
- Errors are logged but no retry
- No fallback mechanism
- Single failure = no scaling for 5 minutes

**Location**: `topic_manager.go:294-300`

**Impact**: Scaling may be delayed significantly

**Recommendation**:
- Retry queue depth check with exponential backoff
- Use cached value if check fails
- Alert on repeated failures

### 12. **Worker Startup Failure Handling**
**Issue**: If worker startup fails, scaling continues:
- Failed workers are logged but scaling proceeds
- May try to scale up but workers fail to start
- No backoff or retry logic

**Location**: `topic_manager.go:180-191`

**Impact**: May repeatedly try to start workers that fail

**Recommendation**:
- Track worker startup failures
- Back off if multiple workers fail to start
- Alert on startup failure patterns

### 13. **No Metrics Export**
**Issue**: Scaling decisions and queue metrics not exposed:
- No Prometheus metrics for queue depth over time
- No metrics for scaling decisions
- No metrics for worker utilization

**Location**: Entire autoscaling system

**Impact**: Cannot monitor or debug scaling behavior

**Recommendation**:
- Add metrics: `queue_depth`, `worker_count`, `scaling_decisions_total`, `messages_processed_per_worker`
- Export scaling decision reasons
- Track scaling effectiveness

### 14. **Contest Weekend Disables Autoscaling**
**Issue**: Autoscaling is disabled during contest weekends:
- Fixed worker count regardless of load
- May have too many or too few workers
- No flexibility for unexpected load

**Location**: `topic_manager.go:143-147`

**Impact**: May over/under provision during contest weekends

**Recommendation**:
- Keep autoscaling enabled but with higher base workers
- Adjust thresholds rather than disable completely
- Allow manual override if needed

### 15. **No Per-Topic Priority**
**Issue**: All topics scale independently, no prioritization:
- Critical topics (InstanceStore) may starve if other topics scale up
- No resource allocation based on importance

**Location**: Entire system

**Impact**: Less important topics may consume resources needed by critical ones

**Recommendation**:
- Add priority levels to topics
- Allocate resources based on priority
- Consider weighted scaling

### 16. **Scale-Up Calculation Edge Cases**
**Issue**: When calculating `workersToAdd`:
- `currentWorkers * ScaleUpPercent` may be fractional
- `max(1, ...)` ensures at least 1, but may overscale
- No upper bound on workers to add

**Location**: `topic_manager.go:310`

**Impact**: May scale up too aggressively

**Recommendation**:
- Add maximum workers to add per scaling action
- Consider: `min(workersToAdd, maxWorkersPerStep)`
- Cap at reasonable value (e.g., 5-10 workers)

## Suggested Improvements

### Phase 1: Quick Wins (High Impact, Low Effort)
1. Add dead zone/hysteresis (different thresholds for scale-up vs scale-down)
2. Add cooldown period after scaling decisions
3. Track messages processed per worker (basic metric)
4. Add maximum workers per scaling step

### Phase 2: Throughput Tracking (High Impact, Medium Effort)
1. Track messages processed per worker per second
2. Calculate throughput: `messagesPerSecond = processedMessages / timeDelta`
3. Scale based on target throughput: `workersNeeded = targetThroughput / workerThroughput`
4. Add Prometheus metrics for throughput

### Phase 3: Predictive Scaling (Medium Impact, Medium Effort)
1. Track queue depth over time
2. Calculate queue growth rate
3. Scale proactively when growth rate > processing rate
4. Dynamic check interval based on queue depth/growth

### Phase 4: Advanced Features (Low Impact, High Effort)
1. Implement feedback loop to verify scaling effectiveness
2. Machine learning for optimal worker count prediction
3. Resource-aware scaling (CPU, memory, connections)
4. Per-topic priority and resource allocation

## Testing Recommendations

### Unit Tests Needed
- Scaling calculation correctness
- Dead zone logic
- Hysteresis thresholds
- Edge cases (zero workers, max workers, etc.)

### Integration Tests Needed
- End-to-end scaling behavior
- Queue depth measurement accuracy
- Worker startup/scaling coordination
- Error recovery scenarios

### Monitoring/Alerting
- Queue depth trends
- Scaling decision frequency
- Worker utilization
- Scaling effectiveness (did it help?)
- Queue growth rate

## Metrics to Track

### Current Metrics (Missing)
- Queue depth (only checked, not tracked over time)
- Worker count (not exported)
- Scaling decisions (not logged)

### Should Add
- `queue_depth_gauge`: Current queue depth per topic
- `queue_depth_rate`: Queue depth change rate
- `worker_count_gauge`: Current worker count per topic
- `messages_processed_total`: Counter of messages processed per worker
- `message_processing_duration`: Histogram of processing time
- `worker_throughput`: Messages per second per worker
- `scaling_decisions_total`: Counter of scaling decisions with reason
- `scaling_effectiveness`: Did scaling help? (queue depth reduction)

## Configuration Recommendations

### InstanceStore Topic
**Current**:
- ScaleUpThreshold: 1000
- ScaleDownThreshold: 100
- ScaleUpPercent: 0.2
- ScaleDownPercent: 0.1

**Issues**: 
- Very high threshold (1000) means queue must be large before scaling
- Large gap between scale-up (1000) and scale-down (100) prevents oscillation but may overscale

**Recommendation**:
- Lower scale-up threshold: 500-750
- Add hysteresis: scale-down at 50-75
- Consider processing time: InstanceStore is fast, so lower thresholds may be better

### PlayerCrawl Topic
**Current**:
- ScaleUpThreshold: 100
- ScaleDownThreshold: 10
- ScaleUpPercent: 0.2
- ScaleDownPercent: 0.1

**Issues**:
- PlayerCrawl is slow (API calls), so need more workers
- Thresholds seem reasonable but doesn't account for processing time

**Recommendation**:
- Track processing time and adjust thresholds accordingly
- Consider: scale based on `queueDepth * avgProcessingTime / targetLatency`

## Code Examples

### Better Scaling Logic (Pseudocode)
```go
func calculateTargetWorkers(queueDepth int, currentWorkers int, avgProcessingTime time.Duration, messagesPerSecond float64) int {
    // Calculate how many workers needed to clear queue in target time (e.g., 5 minutes)
    targetLatency := 5 * time.Minute
    targetThroughput := float64(queueDepth) / targetLatency.Seconds()
    
    // Current throughput
    currentThroughput := messagesPerSecond * float64(currentWorkers)
    
    if currentThroughput == 0 {
        // No data, scale up conservatively
        return currentWorkers + 1
    }
    
    // Workers needed for target throughput
    workersNeeded := int(math.Ceil(targetThroughput / messagesPerSecond))
    
    // Apply bounds
    workersNeeded = max(workersNeeded, minWorkers)
    workersNeeded = min(workersNeeded, maxWorkers)
    
    return workersNeeded
}
```

### Dead Zone Implementation
```go
type ScalingState struct {
    lastScaleTime    time.Time
    lastQueueDepth   int
    lastScaleDirection string // "up", "down", "none"
    consecutiveChecks int     // How many checks in same direction
}

func shouldScale(queueDepth int, state *ScalingState) (bool, string) {
    now := time.Now()
    
    // Cooldown period: don't scale again within 2 minutes
    if now.Sub(state.lastScaleTime) < 2*time.Minute {
        return false, "cooldown"
    }
    
    // Dead zone: require N consecutive checks in same direction
    if queueDepth > scaleUpThreshold {
        if state.lastScaleDirection == "up" {
            state.consecutiveChecks++
        } else {
            state.consecutiveChecks = 1
            state.lastScaleDirection = "up"
        }
        if state.consecutiveChecks >= 2 { // Require 2 checks (10 minutes)
            return true, "scale_up"
        }
    } else if queueDepth < scaleDownThreshold {
        if state.lastScaleDirection == "down" {
            state.consecutiveChecks++
        } else {
            state.consecutiveChecks = 1
            state.lastScaleDirection = "down"
        }
        if state.consecutiveChecks >= 3 { // Require 3 checks (15 minutes) for scale-down
            return true, "scale_down"
        }
    } else {
        state.consecutiveChecks = 0
        state.lastScaleDirection = "none"
    }
    
    return false, "stable"
}
```


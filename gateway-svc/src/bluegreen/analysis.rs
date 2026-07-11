use crate::bluegreen::config::AnalysisConfig;

#[derive(Debug, PartialEq, Eq)]
pub enum Verdict {
    Continue,
    Promote,
    Rollback,
    Extend,
}

/// Pure decision: given the candidate's request counters, how long the analysis
/// has run (excluding extend windows), and how many extends were already used,
/// decide what the controller should do next.
pub fn decide(
    total: u64,
    fail: u64,
    elapsed_secs: u64,
    extends_used: u32,
    a: &AnalysisConfig,
) -> Verdict {
    let fail_rate = if total > 0 {
        fail as f64 / total as f64
    } else {
        0.0
    };
    let max_fail = 1.0 - a.success_threshold;

    // Early rollback: enough samples AND fail rate above threshold — don't wait
    // for the full window.
    if total >= a.min_requests && fail_rate > max_fail {
        return Verdict::Rollback;
    }
    if elapsed_secs < a.window_seconds {
        return Verdict::Continue;
    }
    // Window elapsed:
    if total >= a.min_requests {
        return Verdict::Promote; // success >= threshold (rollback already ruled out)
    }
    // Low traffic:
    if extends_used < a.max_window_multiplier.saturating_sub(1) {
        Verdict::Extend
    } else {
        Verdict::Promote // low-confidence, no failures observed
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn cfg() -> AnalysisConfig {
        AnalysisConfig {
            window_seconds: 60,
            success_threshold: 0.99,
            min_requests: 20,
            max_window_multiplier: 3,
            failure_status_from: 500,
        }
    }

    #[test]
    fn early_rollback_on_high_fail() {
        // 100 req, 5 fail = 5% > 1% threshold, enough samples, even before window ends
        assert_eq!(decide(100, 5, 10, 0, &cfg()), Verdict::Rollback);
    }
    #[test]
    fn continue_within_window() {
        assert_eq!(decide(100, 0, 10, 0, &cfg()), Verdict::Continue);
    }
    #[test]
    fn promote_after_window_healthy() {
        assert_eq!(decide(100, 0, 60, 0, &cfg()), Verdict::Promote);
        assert_eq!(decide(100, 1, 60, 0, &cfg()), Verdict::Promote); // 1% == threshold, not over
    }
    #[test]
    fn extend_when_low_traffic() {
        assert_eq!(decide(5, 0, 60, 0, &cfg()), Verdict::Extend);
    }
    #[test]
    fn low_confidence_promote_after_extends() {
        // extend quota exhausted (max_mult-1 = 2) but still few samples, no failures
        assert_eq!(decide(5, 0, 60, 2, &cfg()), Verdict::Promote);
    }
    #[test]
    fn low_traffic_high_fail_stays_extend_until_min() {
        // few samples (< min=20): can't rollback yet, still Extend
        assert_eq!(decide(5, 5, 60, 0, &cfg()), Verdict::Extend);
    }
}

const CHINA_TIMEZONE_OFFSET_MINUTES = 8 * 60

function chinaMidnightTimestamp(date: string, dayOffset = 0): number {
  const [year, month, day] = date.split('-').map(Number)
  return Math.floor(
    (Date.UTC(year, month - 1, day + dayOffset) -
      CHINA_TIMEZONE_OFFSET_MINUTES * 60 * 1000) /
      1000
  )
}

export function getCommissionTimeRange(startDate: string, endDate: string) {
  return {
    startTime: startDate ? chinaMidnightTimestamp(startDate) : 0,
    // The backend uses an exclusive upper bound. An empty end date means
    // there is no upper bound, which naturally includes records through now.
    endTime: endDate ? chinaMidnightTimestamp(endDate, 1) : 0,
  }
}

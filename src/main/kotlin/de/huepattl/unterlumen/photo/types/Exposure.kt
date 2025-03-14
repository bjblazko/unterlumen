package de.huepattl.unterlumen.photo.types

data class Exposure(
    val time: ExposureTime? = null,
    val mode: String? = null,
    val meteringMode: String? = null,
    val compensation: ExposureCompensation? = null,
    val iso: Int? = null,
)

class ExposureTime(value: Fraction, unit: Unit) : Measurement<Fraction, ExposureTime.Unit>(value, unit) {

    enum class Unit(private val displayName: String) {
        MILLISECONDS("ms"),
        SECONDS("sec"),
        MINUTES("min");

        override fun toString() = displayName
    }

    companion object {

        fun of(numerator: Int, denominator: Int, unit: Unit = Unit.SECONDS) =
            ExposureTime(Fraction.of(numerator, denominator), unit)

        fun of(value: Int, unit: Unit = Unit.SECONDS) =
            ExposureTime(Fraction.of(value, 1), unit)

        /**
         * Parses strings like "1/250 s" or "1 min".
         */
        fun of(str: String): ExposureTime {
            val parts = str.split(" ")
            require(parts.size == 2) { "invalid format, expected '<fraction> <unit>'" }

            val fraction = Fraction.of(parts[0])
            val unit = when (parts[1].lowercase()) {
                "msec", "ms", "millisecond", "milliseconds" -> Unit.MILLISECONDS
                "sec", "s", "second", "seconds" -> Unit.SECONDS
                "min", "m", "minute", "minutes" -> Unit.MINUTES
                else -> throw IllegalArgumentException("unknown unit: ${parts[1]}")
            }

            return ExposureTime(fraction, unit)
        }

    }

}

/**
 * Exposure Compensation or Bias: Supports EV and Stops.
 */
class ExposureCompensation(value: Fraction, unit: Unit) :
    Measurement<Fraction, ExposureCompensation.Unit>(value, unit) {
    enum class Unit { EV, STOPS }

    companion object {

        fun of(numerator: Int, denominator: Int, unit: Unit = Unit.EV) =
            ExposureCompensation(Fraction.of(numerator, denominator), unit)

        fun of(value: Int, unit: Unit = Unit.EV) =
            ExposureCompensation(Fraction.of(value, 1), unit)

        /**
         * Parses strings like "-1/3 EV" or "1 STOP".
         */
        fun of(str: String): ExposureCompensation {
            val parts = str.split(" ")
            require(parts.size == 2) { "invalid format, expected '<fraction> <unit>'" }

            val fraction = Fraction.of(parts[0])
            val unit = when (parts[1].lowercase()) {
                "ev" -> Unit.EV
                "s", "stop", "stops" -> Unit.STOPS
                else -> throw IllegalArgumentException("unknown unit: ${parts[1]}")
            }

            return ExposureCompensation(fraction, unit)
        }

    }

}
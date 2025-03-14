package de.huepattl.unterlumen.photo.types

/**
 * Represents a fraction (e.g., 1/3 or -2/5).
 */
data class Fraction(val numerator: Int, val denominator: Int) {

    init {
        require(denominator > 0) { "denominator must be greater than zero" }
    }

    override fun toString(): String = if (denominator == 1) "$numerator" else "$numerator/$denominator"

    companion object {

        fun of(numerator: Int, denominator: Int) = Fraction(numerator, denominator)

        fun of(value: Int) = Fraction(value, 1)

        /**
         * Parses a fraction string like "1/250" or "1" into a Fraction object.
         */
        fun of(value: String): Fraction {
            return if ("/" in value) {
                val (num, denom) = value.split("/").map { it.toInt() }
                Fraction(num, denom)
            } else {
                Fraction(value.toInt(), 1) // Treat single numbers as "/1"
            }
        }

    }
}

/**
 * Generic class for measurements.
 */
sealed class Measurement<T : Any, U : Enum<U>>(val value: T, val unit: U) {
    override fun toString(): String = "$value $unit"
}
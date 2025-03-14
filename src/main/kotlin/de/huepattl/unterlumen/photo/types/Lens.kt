package de.huepattl.unterlumen.photo.types

import java.math.BigDecimal
import java.math.RoundingMode

data class Lens(
    val brand: String? = null,
    val model: String? = null,
    val aperture: Aperture? = null,
    val focalLength: Length? = null,
) {

    companion object {
        override fun toString(): String {
            return super.toString()
        }
    }
}

data class Aperture(val value: BigDecimal) {

    init {
        require(value == null || value > BigDecimal.ZERO) { "aperture must be positive" }
    }

    fun formatted(): BigDecimal? = value.setScale(1, RoundingMode.HALF_UP)

    override fun toString() = formatted()?.let { "f/$it" } ?: "unknown"

    companion object {

        fun of(str: String): Aperture {
            return if (str.startsWith("f")) {
                Aperture(BigDecimal(str.removePrefix("f/").replace(",", ".")))
            } else {
                Aperture(BigDecimal(str.replace(",", ".")))
            }
        }
    }

}
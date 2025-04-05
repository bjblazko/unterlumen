package de.huepattl.unterlumen.photo.types

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.Assertions.*
import java.math.BigDecimal

class GeoLocationTest {

    @Test
    fun `parse valid geo coordinate`() {
        // given
        val coord = "50° 39' 55,06\""

        // when
        val parsed = GeoCoordinate.parse(coord)

        // then
        assertEquals(50, parsed.degrees)
        assertEquals(39, parsed.minutes)
        assertEquals(BigDecimal("55.06"), parsed.seconds)
    }

    @Test
    fun `parse formally valid geo coordinate but with invalid range`() {
        // given
        val coord = "350° 39' 55,06\"" // degrees must be between -90 and 90

        // when + then
        assertThrows(IllegalArgumentException::class.java) {
            GeoCoordinate.parse(coord)
        }
    }

    @Test
    fun `parse invalid string`() {
        // given
        val coord = "all your base are belong to us"

        // when + then
        assertThrows(IllegalArgumentException::class.java) {
            GeoCoordinate.parse(coord)
        }
    }

}
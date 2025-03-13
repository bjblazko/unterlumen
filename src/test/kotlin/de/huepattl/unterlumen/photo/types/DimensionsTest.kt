package de.huepattl.unterlumen.photo.types

import org.junit.jupiter.api.Assertions.*
import org.junit.jupiter.api.Test

class DimensionsTest {

    @Test
    fun `length pixel`() {
        val px32 = Length.of(32)
        assertEquals("32 px", px32.toString())
    }

    @Test
    fun `length centimeter`() {
        val px32 = Length(20, Length.Unit.CENTIMETERS)
        assertEquals("20 cm", px32.toString())
    }

}
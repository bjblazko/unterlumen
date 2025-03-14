package de.huepattl.unterlumen.photo.types

import org.junit.jupiter.api.Assertions.*
import org.junit.jupiter.api.Test
import java.math.BigDecimal

class LensTest {

    @Test
    fun `aperture to string`() {
        val aperture = Aperture(BigDecimal("5.600001"))
        assertEquals("f/5.6", aperture.toString())
    }

    @Test
    fun `aperture from string`() {
        val aperture = Aperture.of("f/5.600001")
        assertEquals("f/5.6", aperture.toString())
    }

}
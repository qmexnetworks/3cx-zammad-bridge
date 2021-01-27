def parse_phone_number(number: str):
    """
    Normalizes the phone number with international prefix as defined in the config.

    The phone number is expected to be within (), and also expected to be the ONLY thing within ()
    :return: str
    """

    number = number[number.find("(") + 1:number.rfind(")")]

    # TODO what if +49/Germany isn't the default? See 3CX settings.
    # old filter, should better be done in 3CX E.164
    if number.startswith("+49"):
        number = "0" + str(number[3:])
    elif number.startswith("49"):
        number = "0" + str(number[2:])

    return number

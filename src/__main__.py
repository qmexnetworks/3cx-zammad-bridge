#!/usr/bin/python3
import signal
import sys
import uuid
import time

from api3cx.api3cx import Api3CX
from api3cx.call import Call3CX
from bridge_helpers.helpers import parse_phone_number
from config.config import BridgeConfig
from zammad.call import CallZammad
from zammad.zammad import Zammad


def signal_handler(signal, frame):
    print('Received SIGINT, exiting')
    sys.exit(0)


signal.signal(signal.SIGINT, signal_handler)

config = BridgeConfig("/etc/3cx-zammad-bridge/config.yaml",
                      "/opt/3cx-zammad-bridge/config.yaml",
                      "config.yaml"
                      )
session3cx = Api3CX(config)
zammad = Zammad(config.zammad_endpoint)

calls = dict()
while session3cx.is_authenticated:
    new_calls = []
    data = session3cx.fetch_active_calls()

    for row in data['list']:
        row = Call3CX(row)
        direction = ""
        caller = row.Caller.split(" ", 1)
        callee = row.Callee.split(" ", 1)
        call_id = row.Id

        if len(caller[0]) == config.api3CX_extension_digits and \
                len(callee[0]) == config.api3CX_trunk_digits and \
                int(caller[0]) in session3cx.extensions:
            direction = "Outbound"
            agent_number = caller[0]
            agent_name = caller[1]

            phone_number = parse_phone_number(row.Callee)
            source = agent_number
            dest = phone_number
        elif len(caller[0]) == config.api3CX_trunk_digits and \
                len(callee[0]) == config.api3CX_extension_digits and \
                int(callee[0]) in session3cx.extensions:
            direction = "Inbound"
            agent_number = callee[0]
            agent_name = callee[1]

            phone_number = parse_phone_number(row.Caller)
            source = phone_number
            dest = agent_number
        else:
            continue

        if call_id not in calls:
            # Not known calls
            if row.Status == "Routing" or row.Status == "Transferring":
                # This is a new Call Ringing
                call_uid = uuid.uuid4()
                calls[call_id] = CallZammad()
                calls[call_id].callid = call_uid
                calls[call_id].direction = direction
                calls[call_id].number = phone_number
                calls[call_id].status = row.Status
                if direction == "Outbound":
                    calls[call_id].agent = agent_number
                    calls[call_id].agent_name = agent_name

                # Mark only non transferring Calls (Queue Calls) ans newCall
                if row.Status == "Routing":
                    print("New Call with ID " + str(call_uid) + " " + str(direction) + " from " + str(
                        source) + " to " + str(dest))
                    zammad.new_call(calls[call_id])
                elif row.Status == "Transferring":
                    print("New Queue Call with ID " + str(call_uid) + " " + str(direction) + " from " + str(
                        source) + " to " + str(dest))
        else:
            # Already known calls
            # if status was Routing and is now Talking, call is answered
            if calls[call_id].status == "Routing" and row.Status == "Talking":
                calls[call_id].agent = agent_number
                calls[call_id].agent_name = agent_name
                calls[call_id].status = row.Status
                print("Call with ID " + str(calls[call_id].callid) + " " + str(direction) + " from " + str(
                    source) + " was answered by " + str(dest))
                zammad.answer(calls[call_id])
            # if status was Transferring and is now Talking, Queue Call was answered
            if calls[call_id].status == "Transferring" and row.Status == "Talking":
                calls[call_id].agent = agent_number
                calls[call_id].agent_name = agent_name
                calls[call_id].status = row.Status
                print("Queue Call with ID " + str(calls[call_id].callid) + " " + str(direction) + " from " + str(
                    source) + " was answered by " + str(dest))
                zammad.new_call(calls[call_id])
                zammad.answer(calls[call_id])
        new_calls.append(call_id)

    # Clean everything up
    ended_call_ids = []
    for call_id in calls:
        if call_id not in new_calls:
            # If call was Routing its now unanswered
            if calls[call_id].status == "Routing":
                print("Call with ID " + str(calls[call_id].callid) + " " + str(
                    calls[call_id].direction) + " was not answered")
                calls[call_id].cause = "cancel"
                zammad.hangup(calls[call_id])
            elif calls[call_id].status == "Talking":
                print("Call with ID " + str(calls[call_id].callid) + " " + str(
                    calls[call_id].direction) + " was hangup")
                calls[call_id].cause = "normalClearing"
                zammad.hangup(calls[call_id])
            elif calls[call_id].status == "Transferring" and config.zammad_log_missed_queue_calls:
                print("Queue Call with ID " + str(calls[call_id].callid) + " " + str(
                    calls[call_id].direction) + " was not answered")
                calls[call_id].cause = "cancel"
                calls[call_id].agent = config.api3CX_queue_extension
                zammad.new_call(calls[call_id])
                zammad.hangup(calls[call_id])

            ended_call_ids.append(call_id)

    # Delete them now, because we cannot do that while iterating
    for call_id in ended_call_ids:
        del calls[call_id]

    time.sleep(0.5)
